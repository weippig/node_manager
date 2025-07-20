package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	_ "github.com/mattn/go-sqlite3"
)

// --- Structs and Globals ---

type Node struct {
	ID         int
	Name       string
	IP         string
	MacAddress string
	Username   string
	Password   string
}

var content *fyne.Container
var db *sql.DB

// --- Database & File I/O Functions ---

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./nodes.db")
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}

	statement, err := db.Prepare(`
		CREATE TABLE IF NOT EXISTS nodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			mac_address TEXT,
			ip TEXT NOT NULL,
			username TEXT NOT NULL,
			password TEXT NOT NULL
		);
	`)
	if err != nil {
		log.Fatal("Failed to prepare table creation:", err)
	}
	_, err = statement.Exec()
	if err != nil {
		log.Fatal("Failed to execute table creation:", err)
	}
}

func loadNodes() []Node {
	rows, err := db.Query("SELECT id, name, mac_address, ip, username, password FROM nodes ORDER BY name ASC")
	if err != nil {
		log.Println("Failed to query nodes:", err)
		return []Node{} // Return empty slice on error
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.Name, &n.MacAddress, &n.IP, &n.Username, &n.Password); err != nil {
			log.Println("Failed to scan node row:", err)
			continue
		}
		nodes = append(nodes, n)
	}
	return nodes
}

func loadScripts() []string {
	scriptDir := "script"
	files, err := os.ReadDir(scriptDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{} // Directory doesn't exist yet, return empty
		}
		log.Println("Failed to read script directory:", err)
		return []string{}
	}

	var scripts []string
	for _, file := range files {
		if !file.IsDir() {
			scripts = append(scripts, file.Name())
		}
	}
	return scripts
}

// --- Screen Management ---

func setScreen(c fyne.CanvasObject) {
	content.Objects = []fyne.CanvasObject{c}
	content.Refresh()
}

func main() {
	initDB()
	defer db.Close()

	myApp := app.New()
	myWindow := myApp.NewWindow("Node & Script Manager")
	myWindow.Resize(fyne.NewSize(600, 400))

	content = container.New(layout.NewStackLayout())

	// Create screen instances
	addNodeScreen := makeAddNodeScreen()
	addScriptScreen := makeAddScriptScreen()

	sidebar := container.NewVBox(
		widget.NewButton("首頁", func() { setScreen(makeExecuteScreen()) }), // Re-create to refresh data
		widget.NewButton("添加節點", func() { setScreen(addNodeScreen) }),
		widget.NewButton("添加腳本", func() { setScreen(addScriptScreen) }),
	)

	split := container.NewHSplit(sidebar, content)
	split.Offset = 0.2

	setScreen(makeExecuteScreen()) // Set initial screen

	myWindow.SetContent(split)
	myWindow.ShowAndRun()
}

// --- Screen Definitions ---

func makeAddNodeScreen() fyne.CanvasObject {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("e.g., Web Server 1")
	macAddressEntry := widget.NewEntry()
	macAddressEntry.SetPlaceHolder("e.g., 24:5e:be:84:c4:9e")
	ipEntry := widget.NewEntry()
	ipEntry.SetPlaceHolder("e.g., 192.168.1.100")
	userEntry := widget.NewEntry()
	userEntry.SetPlaceHolder("e.g., admin")
	passEntry := widget.NewPasswordEntry()
	passEntry.SetPlaceHolder("Password")

	statusLabel := widget.NewLabel("")

	form := widget.NewForm(
		&widget.FormItem{Text: "Node Name", Widget: nameEntry},
		&widget.FormItem{Text: "IP Address", Widget: ipEntry},
		&widget.FormItem{Text: "Mac Address", Widget: macAddressEntry},
		&widget.FormItem{Text: "Username", Widget: userEntry},
		&widget.FormItem{Text: "Password", Widget: passEntry},
	)

	form.OnSubmit = func() {
		statement, err := db.Prepare("INSERT INTO nodes (name, mac_address, ip, username, password) VALUES (?, ?, ?, ?, ?)")
		if err != nil {
			log.Println("DB prepare error:", err)
			statusLabel.SetText("Database error.")
			return
		}
		defer statement.Close()

		_, err = statement.Exec(nameEntry.Text, macAddressEntry.Text, ipEntry.Text, userEntry.Text, passEntry.Text)
		if err != nil {
			log.Println("DB exec error:", err)
			statusLabel.SetText("Error: Node name may already exist.")
			return
		}

		statusLabel.SetText(fmt.Sprintf("Node '%s' saved.", nameEntry.Text))
		nameEntry.SetText("")
		ipEntry.SetText("")
		macAddressEntry.SetText("")
		userEntry.SetText("")
		passEntry.SetText("")
		form.Refresh()
	}

	return container.NewBorder(
		nil,
		container.NewVBox(widget.NewButton("返回", func() { setScreen(makeExecuteScreen()) }), statusLabel),
		nil, nil,
		form,
	)
}

func makeAddScriptScreen() fyne.CanvasObject {
	filenameEntry := widget.NewEntry()
	filenameEntry.SetPlaceHolder("e.g., my_script.sh")

	contentEntry := widget.NewMultiLineEntry()
	contentEntry.SetPlaceHolder("#!/bin/bash\necho \"Hello from script\"\n")

	statusLabel := widget.NewLabel("")

	form := widget.NewForm(
		&widget.FormItem{Text: "Script Filename", Widget: filenameEntry},
		&widget.FormItem{Text: "Script Content", Widget: contentEntry},
	)

	form.OnSubmit = func() {
		scriptDir := "script"
		if err := os.MkdirAll(scriptDir, 0755); err != nil {
			statusLabel.SetText("Error creating directory.")
			return
		}
		filePath := filepath.Join(scriptDir, filenameEntry.Text)
		if err := os.WriteFile(filePath, []byte(contentEntry.Text), 0644); err != nil {
			statusLabel.SetText("Error saving file.")
			return
		}
		statusLabel.SetText(fmt.Sprintf("Saved to %s", filePath))
		filenameEntry.SetText("")
		contentEntry.SetText("")
	}

	return container.NewBorder(
		nil,
		container.NewVBox(widget.NewButton("返回", func() { setScreen(makeExecuteScreen()) }), statusLabel),
		nil, nil,
		form,
	)
}

func makeExecuteScreen() fyne.CanvasObject {
	// --- Data Loading ---
	allNodes := loadNodes()
	scriptFiles := loadScripts()

	if len(allNodes) == 0 {
		return container.NewCenter(widget.NewLabel("No nodes found. Please add a node first."))
	}

	// Map to keep full node data accessible from the string representation
	nodeMap := make(map[string]Node)
	var nodeOptions []string
	for _, node := range allNodes {
		label := fmt.Sprintf("%s (%s)", node.Name, node.IP)
		nodeOptions = append(nodeOptions, label)
		nodeMap[label] = node
	}

	// --- UI Widgets ---
	nodesCheck := widget.NewCheckGroup(nodeOptions, nil)
	scriptSelect := widget.NewSelect(scriptFiles, nil)
	statusLabel := widget.NewLabel("Ready. Select nodes and a script.")
	statusLabel.Wrapping = fyne.TextWrapWord

	var runButton *widget.Button
	runButton = widget.NewButton("Run Script on Selected Nodes", func() {
		selectedNodeLabels := nodesCheck.Selected
		selectedScript := scriptSelect.Selected

		if len(selectedNodeLabels) == 0 || selectedScript == "" {
			statusLabel.SetText("Error: Must select nodes and a script.")
			return
		}

		runButton.Disable()
		go func() {
			defer fyne.Do(func() {
				runButton.Enable()
			}) // Ensure button is re-enabled on main thread

			for _, label := range selectedNodeLabels {
				node := nodeMap[label] // Get the full node data
				fyne.Do(func() {
					statusLabel.SetText(fmt.Sprintf("Simulating '%s' on '%s'...", selectedScript, node.Name))
				}) // Update status on main thread
				time.Sleep(1 * time.Second) // Simulate work
			}

			fyne.Do(func() {
				statusLabel.SetText("All tasks completed successfully!")
			}) // Final status update on main thread
		}()
	})

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
