package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

func runGUI() {
	var mw *walk.MainWindow
	var sizeEdit, seedEdit, outEdit *walk.LineEdit
	var logEdit *walk.TextEdit
	var generateBtn, openBtn *walk.PushButton

	appendLog := func(format string, args ...any) {}
	setRunning := func(running bool) {}

	appendLog = func(format string, args ...any) {
		text := fmt.Sprintf(format, args...)
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		if mw == nil || logEdit == nil {
			return
		}
		mw.Synchronize(func() {
			logEdit.AppendText(strings.ReplaceAll(text, "\n", "\r\n"))
		})
	}

	var lastOutput string
	var mu sync.Mutex
	setRunning = func(running bool) {
		if mw == nil {
			return
		}
		mw.Synchronize(func() {
			mu.Lock()
			hasOutput := lastOutput != ""
			mu.Unlock()
			generateBtn.SetEnabled(!running)
			openBtn.SetEnabled(!running && hasOutput)
			sizeEdit.SetEnabled(!running)
			seedEdit.SetEnabled(!running)
			outEdit.SetEnabled(!running)
			if running {
				generateBtn.SetText("Generating...")
			} else {
				generateBtn.SetText("Generate map")
			}
		})
	}

	startGeneration := func() {
		size, err := strconv.Atoi(strings.TrimSpace(sizeEdit.Text()))
		if err != nil {
			walk.MsgBox(mw, "Invalid size", "World size must be a number.", walk.MsgBoxIconWarning)
			return
		}
		seed, err := strconv.ParseInt(strings.TrimSpace(seedEdit.Text()), 10, 64)
		if err != nil {
			walk.MsgBox(mw, "Invalid seed", "Seed must be a number.", walk.MsgBoxIconWarning)
			return
		}
		output := strings.TrimSpace(outEdit.Text())
		if output == "" {
			output = defaultOutputPath(size, seed)
			outEdit.SetText(output)
		}
		output = filepath.Clean(output)

		logEdit.SetText("")
		mu.Lock()
		lastOutput = ""
		mu.Unlock()
		setRunning(true)

		go func() {
			started := time.Now()
			appendLog("Rust Map Generator")
			appendLog("Offline mode: generating terrain, roads, monuments and prefabs without RustDedicated.")
			appendLog("Size=%d Seed=%d", size, seed)
			appendLog("")

			result, err := GenerateOfflineMap(OfflineGenConfig{
				Size: size,
				Seed: seed,
				Out:  output,
				Logf: appendLog,
			})
			if err != nil {
				appendLog("")
				appendLog("ERROR: %v", err)
				setRunning(false)
				mw.Synchronize(func() {
					walk.MsgBox(mw, "Generation failed", err.Error(), walk.MsgBoxIconError)
				})
				return
			}

			mu.Lock()
			lastOutput = result.OutputPath
			mu.Unlock()
			appendLog("")
			appendLog("DONE in %s", time.Since(started).Round(time.Second))
			appendLog("Output: %s", result.OutputPath)
			if result.Summary != "" {
				appendLog("")
				appendLog("%s", result.Summary)
			}
			setRunning(false)
			mw.Synchronize(func() {
				walk.MsgBox(mw, "Map generated", "Saved:\n"+result.OutputPath, walk.MsgBoxIconInformation)
			})
		}()
	}

	browseOutput := func() {
		dlg := &walk.FileDialog{
			Title:    "Save map as",
			FilePath: strings.TrimSpace(outEdit.Text()),
			Filter:   "Rust map files (*.map)|*.map|All files (*.*)|*.*",
		}
		if ok, err := dlg.ShowSave(mw); err != nil {
			walk.MsgBox(mw, "Save dialog failed", err.Error(), walk.MsgBoxIconError)
		} else if ok {
			outEdit.SetText(dlg.FilePath)
		}
	}

	openOutput := func() {
		mu.Lock()
		target := lastOutput
		mu.Unlock()
		if target == "" {
			target = strings.TrimSpace(outEdit.Text())
		}
		if target == "" {
			return
		}
		abs, _ := filepath.Abs(target)
		_ = exec.Command("explorer", "/select,"+abs).Start()
	}

	err := MainWindow{
		AssignTo: &mw,
		Title:    "Rust Map Generator",
		MinSize:  Size{Width: 820, Height: 620},
		Size:     Size{Width: 920, Height: 680},
		Layout:   VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 10},
		Children: []Widget{
			GroupBox{
				Title:  "Map settings",
				Layout: Grid{Columns: 3, Spacing: 8},
				Children: []Widget{
					Label{Text: "World size"},
					LineEdit{AssignTo: &sizeEdit, Text: "1500"},
					Label{Text: ""},

					Label{Text: "Seed"},
					LineEdit{AssignTo: &seedEdit, Text: "1500111"},
					Label{Text: ""},

					Label{Text: "Output .map"},
					LineEdit{AssignTo: &outEdit, Text: defaultOutputPath(1500, 1500111)},
					PushButton{Text: "Browse", OnClicked: browseOutput},
				},
			},
			Composite{
				Layout: HBox{MarginsZero: true, Spacing: 8},
				Children: []Widget{
					PushButton{AssignTo: &generateBtn, Text: "Generate map", MinSize: Size{Width: 140, Height: 32}, OnClicked: startGeneration},
					PushButton{AssignTo: &openBtn, Text: "Open output", MinSize: Size{Width: 120, Height: 32}, Enabled: false, OnClicked: openOutput},
					HSpacer{},
				},
			},
			TextEdit{
				AssignTo:      &logEdit,
				ReadOnly:      true,
				VScroll:       true,
				HScroll:       true,
				StretchFactor: 1,
				Text:          "Ready.\r\n",
			},
		},
	}.Create()
	if err != nil {
		walk.MsgBox(nil, "Rust Map Generator", err.Error(), walk.MsgBoxIconError)
		return
	}

	mw.Run()
}
