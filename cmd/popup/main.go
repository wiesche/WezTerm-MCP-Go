package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"wezterm-mcp-go/config"
	"wezterm-mcp-go/paneops"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// ApprovalResult is the JSON response returned to the MCP server.
type ApprovalResult struct {
	PaneID         int    `json:"pane_id"`
	AutoSelected   bool   `json:"auto_selected"`
	Message        string `json:"message"`
	ApprovedByUser bool   `json:"approved_by_user"`
}

// RejectionResult is returned when user rejects.
type RejectionResult struct {
	Error string `json:"error"`
}

func main() {
	// Load config (same directory as popup exe)
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(2)
	}

	// Parse CLI args: popup <text> <pane_id>
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <text> <pane_id>\n", os.Args[0])
		os.Exit(2)
	}

	text := os.Args[1]
	paneID, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid pane_id: %v\n", err)
		os.Exit(2)
	}

	// Fetch pane info to detect shell type
	panes, err := paneops.FetchPaneList()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fetch panes: %v\n", err)
		os.Exit(2)
	}

	var paneTitle string
	for _, p := range panes {
		if p.PaneID == paneID {
			paneTitle = p.Title
			break
		}
	}

	shellType := paneops.DetectShellType(paneTitle)

	// Run GUI and get result
	result := runApprovalGUI(text, paneID, shellType, paneTitle)

	// Output JSON to stdout
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal result: %v\n", err)
		os.Exit(2)
	}
	fmt.Println(string(output))

	// Exit code: 0 = approved, 1 = rejected
	if _, rejected := result.(RejectionResult); rejected {
		os.Exit(1)
	}
}

func runApprovalGUI(text string, paneID int, shellType, paneTitle string) interface{} {
	a := app.New()
	w := a.NewWindow("MCP Approval")
	w.Resize(fyne.NewSize(500, 250))

	// Build UI
	textEntry := widget.NewMultiLineEntry()
	textEntry.SetText(text)
	textEntry.SetMinRowsVisible(5)
	textEntry.Wrapping = fyne.TextWrapWord

	infoLabel := widget.NewLabel(fmt.Sprintf("Tool: send_text | Pane: %d | Shell: %s", paneID, shellType))

	edited := false

	// Store result from button callbacks
	var finalResult interface{}

	// Buttons (callbacks set below)
	approveBtn := widget.NewButton(fmt.Sprintf("Approve [%s]", config.Active.Shortcuts.Approve), nil)
	editBtn := widget.NewButton(fmt.Sprintf("Show+Edit [%s]", config.Active.Shortcuts.Edit), nil)
	rejectBtn := widget.NewButton(fmt.Sprintf("Reject [%s]", config.Active.Shortcuts.Reject), nil)

	// Set button callbacks
	approveBtn.OnTapped = func() {
		if edited {
			paneops.SendEnterToPane(paneID, shellType)
		} else {
			paneops.SendTextWithNewline(paneID, text, shellType)
		}
		finalResult = ApprovalResult{
			PaneID:         paneID,
			AutoSelected:   false,
			Message:        fmt.Sprintf("Text sent to pane %d", paneID),
			ApprovedByUser: true,
		}
		a.Quit()
	}

	editBtn.OnTapped = func() {
		edited = true
		paneops.FocusWezTermWindow()
		paneops.ActivatePane(paneID)
		paneops.SendTextToPane(paneID, text)
	}

	rejectBtn.OnTapped = func() {
		if edited {
			paneops.ClearLineInPane(paneID, shellType)
		}
		finalResult = RejectionResult{Error: "Rejected by user"}
		a.Quit()
	}

	// Keyboard shortcuts
	w.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyName(config.Active.Shortcuts.Approve),
		Modifier: 0,
	}, func(_ fyne.Shortcut) {
		approveBtn.OnTapped()
	})

	w.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyName(config.Active.Shortcuts.Reject),
		Modifier: 0,
	}, func(_ fyne.Shortcut) {
		rejectBtn.OnTapped()
	})

	w.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyName(config.Active.Shortcuts.Edit),
		Modifier: 0,
	}, func(_ fyne.Shortcut) {
		editBtn.OnTapped()
	})

	// Layout
	content := container.NewVBox(
		infoLabel,
		widget.NewSeparator(),
		widget.NewLabel("Text to send:"),
		textEntry,
		container.NewHBox(rejectBtn, editBtn, approveBtn),
	)

	w.SetContent(content)

	// Run blocking on main thread
	w.ShowAndRun()

	// Return the stored result
	if finalResult != nil {
		return finalResult
	}
	return RejectionResult{Error: "Dialog closed without action"}
}
