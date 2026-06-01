# RustEdit Go Mapgen

Windows GUI and CLI generator for offline Rust `.map` files.

The generator does not start `RustDedicated` and does not copy a template map. It builds `WorldData` directly: terrain/height/splat/biome/topology/alpha/water maps, monuments, decor/resource prefabs, roads, and powerline paths.

Monument placement uses embedded terrain footprints extracted from the local Rust server bundles (`HeightTexture` + `BlendTexture`) so large prefabs stamp the ground before they are serialized into the map.

## Build exe

```powershell
go run github.com/akavel/rsrc@latest -manifest .\app.manifest -o .\rsrc.syso
go build -ldflags="-H windowsgui" -o .\RustMapGen.exe .
go build -o .\RustMapGenCLI.exe .
```

`rsrc.syso` embeds the Windows Common Controls v6 manifest required by `walk`.
Double-click `RustMapGen.exe` to use the windowed app. No console input is needed.

## CLI Generate

```powershell
.\RustMapGenCLI.exe -size 1500 -seed 1500111 -out .\out\proceduralmap.1500.1500111.284.map
```

Optional:

- `-height-res 1025` overrides height/water resolution
- `-tex-res 1024` overrides splat/biome/alpha/topology resolution

## Inspect

```powershell
.\RustMapGenCLI.exe -inspect "C:\Users\klox4\Downloads\Telegram Desktop\proceduralmap.1500.1500111.284.map"
```

## Notes

For size `1500` the default resolutions match Rust's usual `1025` height map and `1024` texture/topology maps.
