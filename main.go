package main

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// Global container to hold the current screen
var content *fyne.Container

// setScreen updates the main content area with a new screen
func setScreen(c fyne.CanvasObject) {
	content.Objects = []fyne.CanvasObject{c}
	content.Refresh()
}

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("Node & Script Manager")
	myWindow.Resize(fyne.NewSize(600, 400))

	// The main content area, using MaxLayout to fill the space
	content = container.New(layout.NewMaxLayout())

	// --- Create Screens ---
	executeScreen := makeExecuteScreen()
	addNodeScreen := makeAddNodeScreen()
	addScriptScreen := makeAddScriptScreen()

	// --- Create Sidebar ---
	sidebar := container.NewVBox(
		widget.NewButton("首頁", func() {
			setScreen(executeScreen)
		}),
		widget.NewButton("添加節點", func() {
			setScreen(addNodeScreen)
		}),
		widget.NewButton("添加腳本", func() {
			setScreen(addScriptScreen)
		}),
	)

	// --- Create the Main Layout ---
	// HSplit provides a draggable vertical separator
	split := container.NewHSplit(sidebar, content)
	split.Offset = 0.2 // Give the sidebar an initial 20% width

	// Set the initial screen to be the execute screen
	setScreen(executeScreen)

	myWindow.SetContent(split)
	myWindow.ShowAndRun()
}

// makeAddNodeScreen creates the placeholder screen for adding nodes
func makeAddNodeScreen() fyne.CanvasObject {
	return container.NewVBox(
		widget.NewLabel("Hello World - Add Node Screen"),
		widget.NewButton("返回", func() {
			// Return to the main execute screen
			setScreen(makeExecuteScreen())
		}),
	)
}

// makeAddScriptScreen creates the placeholder screen for adding scripts
func makeAddScriptScreen() fyne.CanvasObject {
	return container.NewVBox(
		widget.NewLabel("Hello World - Add Script Screen"),
		widget.NewButton("返回", func() {
			// Return to the main execute screen
			setScreen(makeExecuteScreen())
		}),
	)
}

// makeExecuteScreen creates the main functionality screen
func makeExecuteScreen() fyne.CanvasObject {
	// --- Data (Hardcoded for now) ---
	nodes := []string{"Node A (192.168.1.10)", "Node B (192.168.1.11)", "Node C (192.168.1.12)"}
	scripts := []string{"deploy_app.sh", "check_status.sh", "reboot_server.sh"}

	// --- UI Widgets ---
	nodesCheck := widget.NewCheckGroup(nodes, nil)
	scriptSelect := widget.NewSelect(scripts, func(s string) {})
	statusLabel := widget.NewLabel("Ready. Select nodes and a script.")
	statusLabel.Wrapping = fyne.TextWrapWord

	var runButton *widget.Button
	runButton = widget.NewButton("Run Script on Selected Nodes", func() {
		selectedNodes := nodesCheck.Selected
		selectedScript := scriptSelect.Selected

		if len(selectedNodes) == 0 || selectedScript == "" {
			statusLabel.SetText("Error: Must select nodes and a script.")
			return
		}

		runButton.Disable()
		go func() {
			defer runButton.Enable()
			for _, node := range selectedNodes {
				statusLabel.SetText(fmt.Sprintf("Running '%s' on '%s'...", selectedScript, node))
				time.Sleep(1 * time.Second) // Simulate work
			}
			statusLabel.SetText("All tasks completed successfully!")
		}()
	})

	// --- Layout for this screen ---
	return container.NewVBox(
		widget.NewLabel("1. Select Nodes:"),
		nodesCheck,
		widget.NewSeparator(),
		widget.NewLabel("2. Select Script:"),
		scriptSelect,
		widget.NewSeparator(),
		runButton,
		statusLabel,
	)
}