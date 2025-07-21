package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/ssh"
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
			ip TEXT NOT NULL,
			mac_address TEXT,
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
	rows, err := db.Query("SELECT id, name, ip, mac_address, username, password FROM nodes ORDER BY name ASC")
	if err != nil {
		log.Println("Failed to query nodes:", err)
		return []Node{} // Return empty slice on error
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.Name, &n.IP, &n.MacAddress, &n.Username, &n.Password); err != nil {
			log.Println("Failed to scan node row:", err)
			continue
		}
		nodes = append(nodes, n)
	}
	return nodes
}

func deleteNode(nodeID int) error {
	statement, err := db.Prepare("DELETE FROM nodes WHERE id = ?")
	if err != nil {
		log.Println("DB prepare error (delete node):", err)
		return fmt.Errorf("database error preparing delete: %w", err)
	}
	defer statement.Close()

	_, err = statement.Exec(nodeID)
	if err != nil {
		log.Println("DB exec error (delete node):", err)
		return fmt.Errorf("database error executing delete: %w", err)
	}
	return nil
}

func updateNode(node Node) error {
	statement, err := db.Prepare("UPDATE nodes SET name = ?, ip = ?, mac_address = ?, username = ?, password = ? WHERE id = ?")
	if err != nil {
		log.Println("DB prepare error (update node):", err)
		return fmt.Errorf("database error preparing update: %w", err)
	}
	defer statement.Close()

	_, err = statement.Exec(node.Name, node.IP, node.MacAddress, node.Username, node.Password, node.ID)
	if err != nil {
		log.Println("DB exec error (update node):", err)
		return fmt.Errorf("database error executing update: %w", err)
	}
	return nil
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

// readScriptContent reads the content of a script file from the 'script' directory.
func readScriptContent(filename string) (string, error) {
	filePath := filepath.Join("script", filename)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read script file %s: %w", filename, err)
	}
	return string(content), nil
}

// updateScriptContent writes the new content to a script file in the 'script' directory.
func updateScriptContent(filename, content string) error {
	filePath := filepath.Join("script", filename)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write script file %s: %w", filename, err)
	}
	return nil
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

	content = container.New(layout.NewMaxLayout())

	// Create screen instances
	addNodeScreen := makeAddNodeScreen()
	addScriptScreen := makeAddScriptScreen()

	sidebar := container.NewVBox(
		widget.NewButton("首頁", func() { setScreen(makeExecuteScreen()) }), // Re-create to refresh data
		widget.NewButton("添加節點", func() { setScreen(addNodeScreen) }),
		widget.NewButton("添加腳本", func() { setScreen(addScriptScreen) }),
		widget.NewButton("管理節點", func() { setScreen(makeManageNodesScreen(myWindow)) }), // Pass window for dialogs
		widget.NewButton("管理腳本", func() { setScreen(makeManageScriptsScreen(myWindow)) }), // Pass window for dialogs
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
	ipEntry := widget.NewEntry()
	ipEntry.SetPlaceHolder("e.g., 192.168.1.100")
	macAddressEntry := widget.NewEntry()
	macAddressEntry.SetPlaceHolder("e.g., 00:1A:2B:3C:4D:5E")
	userEntry := widget.NewEntry()
	userEntry.SetPlaceHolder("e.g., admin")
	passEntry := widget.NewPasswordEntry()
	passEntry.SetPlaceHolder("Password")

	statusLabel := widget.NewLabel("")

	form := widget.NewForm(
		&widget.FormItem{Text: "Node Name", Widget: nameEntry},
		&widget.FormItem{Text: "IP Address", Widget: ipEntry},
		&widget.FormItem{Text: "MAC Address", Widget: macAddressEntry},
		&widget.FormItem{Text: "Username", Widget: userEntry},
		&widget.FormItem{Text: "Password", Widget: passEntry},
	)

	form.OnSubmit = func() {
		statement, err := db.Prepare("INSERT INTO nodes (name, ip, mac_address, username, password) VALUES (?, ?, ?, ?, ?)")
		if err != nil {
			log.Println("DB prepare error:", err)
			statusLabel.SetText("Database error.")
			return
		}
		defer statement.Close()

		_, err = statement.Exec(nameEntry.Text, ipEntry.Text, macAddressEntry.Text, userEntry.Text, passEntry.Text)
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

// makeEditNodeScreen creates a screen to edit an existing node.
func makeEditNodeScreen(w fyne.Window, node Node) fyne.CanvasObject {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("e.g., Web Server 1")
	nameEntry.SetText(node.Name)

	ipEntry := widget.NewEntry()
	ipEntry.SetPlaceHolder("e.g., 192.168.1.100")
	ipEntry.SetText(node.IP)

	macAddressEntry := widget.NewEntry()
	macAddressEntry.SetPlaceHolder("e.g., 00:1A:2B:3C:4D:5E")
	macAddressEntry.SetText(node.MacAddress)

	userEntry := widget.NewEntry()
	userEntry.SetPlaceHolder("e.g., admin")
	userEntry.SetText(node.Username)

	passEntry := widget.NewPasswordEntry()
	passEntry.SetPlaceHolder("Password (leave blank to keep current)")
	// We don't pre-fill password for security reasons

	statusLabel := widget.NewLabel("")

	form := widget.NewForm(
		&widget.FormItem{Text: "Node Name", Widget: nameEntry},
		&widget.FormItem{Text: "IP Address", Widget: ipEntry},
		&widget.FormItem{Text: "MAC Address", Widget: macAddressEntry},
		&widget.FormItem{Text: "Username", Widget: userEntry},
		&widget.FormItem{Text: "Password", Widget: passEntry},
	)

	form.OnSubmit = func() {
		updatedNode := Node{
			ID:         node.ID,
			Name:       nameEntry.Text,
			IP:         ipEntry.Text,
			MacAddress: macAddressEntry.Text,
			Username:   userEntry.Text,
			Password:   node.Password, // Default to old password
		}

		// Only update password if a new one is provided
		if passEntry.Text != "" {
			updatedNode.Password = passEntry.Text
		}

		if err := updateNode(updatedNode); err != nil {
			statusLabel.SetText(fmt.Sprintf("Error updating node: %v", err))
			return
		}

		statusLabel.SetText(fmt.Sprintf("Node '%s' updated successfully!", updatedNode.Name))
		// After update, go back to manage nodes screen to refresh list
		setScreen(makeManageNodesScreen(w))
	}

	return container.NewBorder(
		nil,
		container.NewVBox(widget.NewButton("返回", func() { setScreen(makeManageNodesScreen(w)) }), statusLabel),
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

// makeEditScriptScreen creates a screen to edit an existing script.
func makeEditScriptScreen(w fyne.Window, scriptName string) fyne.CanvasObject {
	filenameEntry := widget.NewEntry()
	filenameEntry.SetText(scriptName)
	filenameEntry.Disable() // Filename should not be editable

	contentEntry := widget.NewMultiLineEntry()
	contentEntry.SetPlaceHolder("#!/bin/bash\necho \"Hello from script\"\n")

	// Load existing script content
	if content, err := readScriptContent(scriptName); err == nil {
		contentEntry.SetText(content)
	} else {
		log.Println("Error loading script content for editing:", err)
		dialog.ShowError(fmt.Errorf("failed to load script content: %w", err), w)
	}

	statusLabel := widget.NewLabel("")

	form := widget.NewForm(
		&widget.FormItem{Text: "Script Filename", Widget: filenameEntry},
		&widget.FormItem{Text: "Script Content", Widget: contentEntry},
	)

	form.OnSubmit = func() {
		if err := updateScriptContent(scriptName, contentEntry.Text); err != nil {
			statusLabel.SetText(fmt.Sprintf("Error updating script: %v", err))
			return
		}
		statusLabel.SetText(fmt.Sprintf("Script '%s' updated successfully!", scriptName))
		// After update, go back to manage scripts screen to refresh list
		setScreen(makeManageScriptsScreen(w))
	}

	return container.NewBorder(
		nil,
		container.NewVBox(widget.NewButton("返回", func() { setScreen(makeManageScriptsScreen(w)) }), statusLabel),
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
		selectedScriptFilename := scriptSelect.Selected

		if len(selectedNodeLabels) == 0 || selectedScriptFilename == "" {
			fyne.Do(func() {
				statusLabel.SetText("Error: Must select nodes and a script.")
			})
			return
		}

		scriptContent, err := readScriptContent(selectedScriptFilename)
		if err != nil {
			fyne.Do(func() {
				statusLabel.SetText(fmt.Sprintf("Error reading script: %v", err))
			})
			return
		}

		fyne.Do(func() {
			runButton.Disable()
			statusLabel.SetText("Starting script execution...")
		})

		go func() {
			defer fyne.Do(func() {
				runButton.Enable()
				statusLabel.SetText("All tasks completed. Ready for next execution.")
			})

			for _, label := range selectedNodeLabels {
				node := nodeMap[label] // Get the full node data
				status := ""

				fyne.Do(func() {
					statusLabel.SetText(fmt.Sprintf("Connecting to %s (%s)...", node.Name, node.IP))
				})

				// SSH Client Configuration
				config := &ssh.ClientConfig{
					User: node.Username,
					Auth: []ssh.AuthMethod{
						ssh.Password(node.Password),
					},
					HostKeyCallback: ssh.InsecureIgnoreHostKey(), // WARNING: Insecure for production!
					Timeout:         5 * time.Second,
				}

				client, err := ssh.Dial("tcp", node.IP+":22", config)
				if err != nil {
					status = fmt.Sprintf("Failed to connect to %s: %v", node.Name, err)
					fyne.Do(func() {
						statusLabel.SetText(status)
					})
					time.Sleep(2 * time.Second) // Pause to show error
					continue                     // Move to next node
				}
				defer client.Close()

				session, err := client.NewSession()
				if err != nil {
					status = fmt.Sprintf("Failed to create session on %s: %v", node.Name, err)
					fyne.Do(func() {
						statusLabel.SetText(status)
					})
					time.Sleep(2 * time.Second)
					continue
				}
				defer session.Close()

				// Generate a unique temporary script path on the remote server
				remoteScriptPath := fmt.Sprintf("/tmp/remote_script_%d.sh", time.Now().UnixNano())

				// Construct the full command to upload, execute, and delete the script
				// Using 'EOF_MARKER_UNIQUE' to minimize collision risk with script content
				fullRemoteCommand := fmt.Sprintf(`
cat <<'EOF_MARKER_UNIQUE' > %s
%s
EOF_MARKER_UNIQUE
chmod +x %s
%s
rm %s
`,
					remoteScriptPath, scriptContent, remoteScriptPath, remoteScriptPath, remoteScriptPath)

				fyne.Do(func() {
					statusLabel.SetText(fmt.Sprintf("Uploading and executing '%s' on %s...", selectedScriptFilename, node.Name))
				})

				// Run the combined command
				var stdoutBuf, stderrBuf bytes.Buffer
				session.Stdout = &stdoutBuf
				session.Stderr = &stderrBuf
				err = session.Run(fullRemoteCommand)

				if err != nil {
					status = fmt.Sprintf("Script '%s' on %s failed: %v\nSTDOUT:\n%s\nSTDERR:\n%s",
						selectedScriptFilename, node.Name, err, stdoutBuf.String(), stderrBuf.String())
				} else {
					status = fmt.Sprintf("Script '%s' on %s succeeded.\nSTDOUT:\n%s\nSTDERR:\n%s",
						selectedScriptFilename, node.Name, stdoutBuf.String(), stderrBuf.String())
				}

				fyne.Do(func() {
					statusLabel.SetText(status)
				})
				time.Sleep(3 * time.Second) // Pause to show result for each node
			}
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

// makeManageNodesScreen creates the screen for managing existing nodes.
func makeManageNodesScreen(w fyne.Window) fyne.CanvasObject {
	allNodes := loadNodes()

	if len(allNodes) == 0 {
		return container.NewCenter(
			container.NewVBox(
				widget.NewLabel("No nodes found. Add a node first."),
				widget.NewButton("返回", func() { setScreen(makeExecuteScreen()) }),
			),
		)
	}

	list := widget.NewList(
		func() int { return len(allNodes) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Node Name (IP)"),
				layout.NewSpacer(),
				widget.NewButton("編輯", func() {}),
				widget.NewButton("刪除", func() {}),
			)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			node := allNodes[i]
			rowContent := o.(*fyne.Container)
			
			// Update Label
			label := rowContent.Objects[0].(*widget.Label)
			label.SetText(fmt.Sprintf("%s (%s)", node.Name, node.IP))

			// Edit Button
			editBtn := rowContent.Objects[2].(*widget.Button) // Corrected Index for edit button
			editBtn.SetText("編輯")
			editBtn.OnTapped = func() {
				setScreen(makeEditNodeScreen(w, node)) // Navigate to edit screen
			}

			// Delete Button
			deleteBtn := rowContent.Objects[3].(*widget.Button) // Corrected Index for delete button
			deleteBtn.SetText("刪除")
			deleteBtn.OnTapped = func() {
				dialog.ShowConfirm(
					"Confirm Deletion",
					fmt.Sprintf("Are you sure you want to delete node '%s'?", node.Name),
					func(confirmed bool) {
						if confirmed {
							if err := deleteNode(node.ID); err != nil {
								dialog.ShowError(err, w)
							} else {
								setScreen(makeManageNodesScreen(w)) // Refresh the list
							}
						}
					},
					w,
				)
			}
		},
	)

	return container.NewBorder(
		nil,
		widget.NewButton("返回", func() { setScreen(makeExecuteScreen()) }),
		nil, nil,
		list,
	)
}

// New: makeManageScriptsScreen creates the screen for managing existing scripts.
func makeManageScriptsScreen(w fyne.Window) fyne.CanvasObject {
	scriptFiles := loadScripts()

	if len(scriptFiles) == 0 {
		return container.NewCenter(
			container.NewVBox(
				widget.NewLabel("No scripts found. Add a script first."),
				widget.NewButton("返回", func() { setScreen(makeExecuteScreen()) }),
			),
		)
	}

	list := widget.NewList(
		func() int { return len(scriptFiles) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Script Name"),
				layout.NewSpacer(),
				widget.NewButton("編輯", func() {}),
				widget.NewButton("刪除", func() {}),
			)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			scriptName := scriptFiles[i]
			rowContent := o.(*fyne.Container)

			// Update Label
			label := rowContent.Objects[0].(*widget.Label)
			label.SetText(scriptName)

			// Edit Button
			editBtn := rowContent.Objects[2].(*widget.Button) // Corrected Index for edit button
			editBtn.SetText("編輯")
			editBtn.OnTapped = func() {
				setScreen(makeEditScriptScreen(w, scriptName)) // Navigate to edit screen
			}

			// Delete Button
			deleteBtn := rowContent.Objects[3].(*widget.Button) // Corrected Index for delete button
			deleteBtn.SetText("刪除")
			deleteBtn.OnTapped = func() {
				dialog.ShowConfirm(
					"Confirm Deletion",
					fmt.Sprintf("Are you sure you want to delete script '%s'?", scriptName),
					func(confirmed bool) {
						if confirmed {
							if err := os.Remove(filepath.Join("script", scriptName)); err != nil {
								dialog.ShowError(err, w)
							} else {
								setScreen(makeManageScriptsScreen(w)) // Refresh the list
							}
						}
					},
					w,
				)
			}
		},
	)

	return container.NewBorder(
		nil,
		widget.NewButton("返回", func() { setScreen(makeExecuteScreen()) }),
		nil, nil,
		list,
	)
}