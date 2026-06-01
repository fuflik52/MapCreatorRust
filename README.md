# MapCreatorRust / RustEdit Go Mapgen

Оффлайн-генератор карт Rust в формате `.map` для открытия в RustEdit и дальнейшей работы без запуска `RustDedicated`.

Проект сам собирает `WorldData` и сериализует карту напрямую: terrain/height/water, splat/biome/alpha/topology, prefabs, monuments, roads, powerlines и underground dungeon-объекты. Сервер Rust не участвует в генерации карты, не стартует и не копирует готовую procedural-карту.

## Что умеет проект

- Генерирует Rust `.map` файл полностью локально.
- Работает через CLI и простое Windows GUI.
- Пишет корректный `WorldData.size` в protobuf.
- Масштабирует координаты и количество path nodes под размер карты.
- Создает базовый островной terrain, coastline, water map и surface masks.
- Расставляет монументы, декор, ресурсы, дорожные/ЛЭП prefabs.
- Генерирует road и powerline paths.
- Добавляет underground dungeon / metro prefab-сетку.
- Не прижимает `Dungeon` и `DungeonBase` к поверхности земли.
- Применяет footprint-данные монументов: `Height`, `Alpha`, `Splat`, `Biome`, `Topology`.
- Умеет инспектировать существующие `.map` файлы и выводить краткую структуру.

## Важная идея

Rust server умеет строить procedural world внутри своего пайплайна. Здесь задача другая: получить похожую структуру карты без участия сервера. Поэтому проект содержит собственный генератор и использует серверные данные только как референс для логики и footprint-масок.

Карта не берется из `RustDedicated`, не скачивается и не копируется из шаблона. Она собирается кодом проекта и сохраняется как `.map`.

## Быстрый старт

### Готовая CLI-генерация

```powershell
.\RustMapGenCLI.exe -size 1500 -seed 1500111 -out .\out\proceduralmap.1500.1500111.284.map
```

После выполнения файл появится в `out`.

### GUI

Запустите:

```powershell
.\RustMapGen.exe
```

В окне укажите размер, seed и путь вывода. GUI вызывает тот же оффлайн-генератор, что и CLI.

## CLI параметры

```text
-size int
    Размер мира в Rust map units.
    По умолчанию: 1500.

-seed int
    Procedural seed.
    По умолчанию: 1500111.

-out string
    Путь для сохранения .map файла.
    Если не указан, используется стандартный путь в out.

-height-res int
    Разрешение height/water map.
    По умолчанию: nextPow2(size / 2) + 1.

-tex-res int
    Разрешение splat/biome/alpha/topology maps.
    По умолчанию: nextPow2(size / 2).

-inspect string
    Не генерировать карту, а прочитать существующий .map и вывести краткую структуру.

-offline
    Оставлен для совместимости. Оффлайн-генерация включена всегда.
```

Пример для карты 2000:

```powershell
.\RustMapGenCLI.exe -size 2000 -seed 1500111 -out .\out\sizecheck.2000.1500111.map
```

## Инспектор карт

Инспектор нужен, чтобы быстро проверить, что реально записано внутрь `.map`: version, timestamp, `worldSize`, количество maps, prefabs, paths и количество nodes в каждом path.

```powershell
.\RustMapGenCLI.exe -inspect ".\out\proceduralmap.1500.1500111.284.map"
```

Пример ожидаемого вывода для 1500:

```text
worldSize=1500
maps=7
prefabs=940
paths=5
Road 0: 103 nodes
Road 1: 86 nodes
Road 2: 50 nodes
Powerline 0: 43 nodes
Powerline 1: 92 nodes
```

Пример для 2000:

```text
worldSize=2000
prefabs=1578
paths=5
Road 0: 137 nodes
Road 1: 115 nodes
Road 2: 67 nodes
Powerline 0: 57 nodes
Powerline 1: 123 nodes
```

Главное: `worldSize` должен совпадать с `-size`. Если coordinates/path data рассчитаны под 1500, а `WorldData.size` записан как 2000, вся карта начинает разъезжаться.

## Сборка

Нужен Go на Windows.

### Сборка CLI

```powershell
go build -o .\RustMapGenCLI.exe .
```

### Сборка GUI

GUI использует `walk`, которому нужен Windows manifest для Common Controls v6.

```powershell
go run github.com/akavel/rsrc@latest -manifest .\app.manifest -o .\rsrc.syso
go build -ldflags="-H windowsgui" -o .\RustMapGen.exe .
```

### Полная пересборка

```powershell
go test ./...
go build -o .\RustMapGenCLI.exe .
go build -ldflags="-H windowsgui" -o .\RustMapGen.exe .
```

## Структура проекта

```text
.
├── main.go              CLI entrypoint: flags, inspect/generate режимы
├── offlinegen.go        Обертка оффлайн-генерации и логирование
├── gui.go               Windows GUI на walk
├── generator.go         Главный pipeline генерации WorldData и maps
├── content.go           Prefabs, monuments, roads, powerlines, dungeon, masks
├── footprint.go         Чтение и применение monument footprint файлов
├── protobuf.go          Marshal/inspect protobuf WorldData
├── world.go             Сохранение .map и LZ4 stream wrapper
├── lz4stream.go         Запись LZ4 framed/block stream
├── noise.go             Noise, random helpers, math helpers
├── app.manifest         Windows Common Controls v6 manifest для GUI
├── footprints/          Embedded footprint data для монументов
├── third_party/walk/    Vendored walk GUI toolkit
└── out/                 Локальные сгенерированные карты, не для git
```

## Как устроена генерация

Pipeline находится в `GenerateWorld`:

1. Проверяются `size`, `height-res`, `tex-res`.
2. Генерируется базовый heightfield острова.
3. Генерируются content objects: монументы, roads, powerlines, dungeon, decor/resources.
4. Terrain выравнивается под дороги и монументы.
5. Surface prefabs и path nodes обновляют высоту по terrain.
6. Создаются карты: `terrain`, `height`, `splat`, `biome`, `topology`, `alpha`, `water`.
7. Footprint/path/prefab masks дорисовываются поверх базовых maps.
8. `WorldData` сериализуется в protobuf и пишется в `.map`.

## Maps внутри .map

Проект пишет 7 map layers:

```text
terrain
height
splat
biome
topology
alpha
water
```

### terrain / height

Высотные карты размером `height-res * height-res * 2` байта. Значения пишутся как normalized short.

Для `size=1500` дефолт:

```text
height-res = 1025
terrain bytes = 2101250
height bytes = 2101250
```

### splat

Текстурные веса terrain layers. Размер:

```text
tex-res * tex-res * 8
```

Для `tex-res=1024`:

```text
8388608 bytes
```

### biome

Biome weights. Размер:

```text
tex-res * tex-res * 5
```

Для `tex-res=1024`:

```text
5242880 bytes
```

### topology

Битовая topology map. Размер:

```text
tex-res * tex-res * 4
```

Для `tex-res=1024`:

```text
4194304 bytes
```

Часть используемых topology bits:

```text
Monument  = 1024
Road      = 2048
Roadside  = 4096
Rail      = 524288
Railside  = 1048576
Building  = 2097152
Mainland  = 536870912
```

### alpha

Alpha/visibility mask. Размер:

```text
tex-res * tex-res
```

Для `tex-res=1024`:

```text
1048576 bytes
```

### water

Water height map размером `height-res * height-res * 2` байта.

## Prefabs и категории

Проект пишет prefabs в `WorldData.Prefabs`.

Основные категории:

```text
Monument
Decor
Road
Powerline
Dungeon
DungeonBase
```

Важное поведение:

- Surface prefabs получают `Y` по terrain.
- `Monument` не переснапливается после flattening, чтобы не ломать footprint-базу.
- `Dungeon` и `DungeonBase` не переснапливаются к terrain, потому что они должны быть под землей.
- Для крупных монументов используется `ClearRadius` и footprint data.

## Roads и Powerlines

Paths пишутся в `WorldData.Paths`.

Для 1500 seed `1500111` сейчас структура такая:

```text
Road 0       103 nodes
Road 1        86 nodes
Road 2        50 nodes
Powerline 0   43 nodes
Powerline 1   92 nodes
```

Для 2000 seed `1500111` node count масштабируется:

```text
Road 0       137 nodes
Road 1       115 nodes
Road 2        67 nodes
Powerline 0   57 nodes
Powerline 1  123 nodes
```

Именно это защищает от ситуации, когда path coordinates выглядят как 1500m layout, а `WorldData.size` уже 2000.

## Underground / Metro / Dungeon

Underground-часть создается в `generateDungeonPrefabs`.

Ключевой момент: dungeon prefabs нельзя поднимать на terrain surface. Если их переснапить как обычные камни/декор, метро окажется на земле. Поэтому `refreshContentHeights` пропускает:

```text
Dungeon
DungeonBase
```

Позиции dungeon-объектов остаются ниже terrain.

## Footprints

Папка `footprints` содержит footprint-файлы монументов и `index.json`.

Footprint используется для:

- выравнивания terrain под монумент;
- применения alpha mask;
- применения splat mask;
- применения biome mask;
- применения topology mask;
- расчета зоны очистки/clearance.

Это нужно, чтобы монумент не стоял в воздухе, не проваливался в землю и не имел вокруг случайную траву/лес/неправильную topology.

Формат читает `footprint.go`. По коду проект ожидает бинарный формат `RMFP2`.

## Где что править

### Поменять форму острова / горы / берег

Файл:

```text
generator.go
```

Функции:

```text
generateBaseHeights
terrainHeight
writeSplat
writeBiome
writeTopology
```

### Поменять расположение монументов

Файл:

```text
content.go
```

Функции:

```text
placeMonuments
findBuildSpot
isMonumentBuildable
overlapsClearings
```

### Поменять дороги / ЛЭП

Файл:

```text
content.go
```

Функции:

```text
generatePaths
pathNodesFromAnchors
placePathPrefabs
paintPath
applyRoadFlattening
```

### Поменять метро / underground grid

Файл:

```text
content.go
```

Функция:

```text
generateDungeonPrefabs
```

### Поменять маски монументов

Файлы:

```text
footprint.go
content.go
```

Функции:

```text
getMonumentFootprint
applyMonumentFootprint
paintMonumentFootprintMasks
applyFootprintAlpha
applyFootprintSplat
applyFootprintBiome
```

### Поменять protobuf / inspect

Файл:

```text
protobuf.go
```

Основное:

```text
MarshalWorld
InspectWorldFile
```

### Поменять сохранение .map

Файлы:

```text
world.go
lz4stream.go
```

## Проверка после изменений

Минимальная проверка:

```powershell
go test ./...
go build -o .\RustMapGenCLI.exe .
.\RustMapGenCLI.exe -size 1500 -seed 1500111 -out .\out\test.1500.1500111.map
.\RustMapGenCLI.exe -inspect .\out\test.1500.1500111.map
```

Проверка size scaling:

```powershell
.\RustMapGenCLI.exe -size 2000 -seed 1500111 -out .\out\test.2000.1500111.map
.\RustMapGenCLI.exe -inspect .\out\test.2000.1500111.map
```

Что смотреть:

- `worldSize` совпадает с `-size`.
- Количество path nodes меняется при смене size.
- `Dungeon`/`DungeonBase` не оказываются на поверхности.
- Монументы стоят на выровненной земле.
- В RustEdit нет массовых prefabs в воздухе.
- Вокруг монументов применяются alpha/splat/topology masks.

## .gitignore и артефакты

В git не должны попадать:

```text
out/
*.exe
*.map
```

Карты и exe-файлы являются локальными артефактами сборки/генерации. Исходники, footprint-данные и код генератора должны быть в репозитории.

## Ограничения

Это не официальный Rust world generator и не полная копия server-side procedural pipeline. Проект приближает структуру Rust-карты оффлайн и пишет совместимый `.map`, но часть логики Rust server все еще может отличаться:

- точный placement всех official monuments;
- точная генерация rail/metro layout как на сервере;
- полная parity со всеми версиями Rust;
- все типы prefabs и dungeon-вариантов;
- все внутренние правила costmap/terrain filters.

Цель проекта: автономно создавать рабочую RustEdit-карту без сервера, с корректным размером мира, нормальными paths, подземным метро и применением нужных масок.

## Типичные проблемы

### Карта выглядит как 1500, хотя size 2000

Проверить:

```powershell
.\RustMapGenCLI.exe -inspect .\out\your.map
```

Если `worldSize` не совпадает с нужным размером, значит карта записана неправильно. Если `worldSize` правильный, но paths слишком короткие/центральные, смотреть `generatePaths` и `scaledCount`.

### Метро на поверхности

Проверить, что `refreshContentHeights` не снапит:

```text
Dungeon
DungeonBase
```

Эти категории должны оставаться ниже terrain.

### Монументы стоят в воздухе

Проверить:

- есть ли footprint для prefab ID;
- вызывается ли `applyMonumentFootprint`;
- вызывается ли `paintMonumentFootprintMasks`;
- не перезаписывается ли `Position.Y` после flattening.

### Много prefabs в одном месте

Смотреть:

```text
findBuildSpot
overlapsClearings
randomLandPoint
scatterRandomPrefabs
scatterNearRoads
```

Там отвечают за spacing, blockers и buildable checks.

## Лицензии и third-party

GUI использует vendored `walk` в `third_party/walk`. Лицензия `walk` лежит в:

```text
third_party/walk/LICENSE
```

Перед публикацией релиза проверьте, что лицензии third-party зависимостей сохранены.
