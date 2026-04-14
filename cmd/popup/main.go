package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"wezterm-mcp-go/config"
	"wezterm-mcp-go/paneops"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// ApprovalResult is the minimal JSON returned to the MCP server on approve.
// Output capture and diff is handled by the server after popup exits.
type ApprovalResult struct {
	PaneID         int   `json:"pane_id"`
	ApprovedByUser bool  `json:"approved_by_user"`
	TimeElapsedMs  int64 `json:"time_elapsed_ms,omitempty"`
}

// RejectionResult is returned when user rejects.
type RejectionResult struct {
	Error string `json:"error"`
}

func main() {
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(2)
	}

	// Parse CLI args: popup <text> <pane_id> [wait_ms]
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <text> <pane_id> [wait_ms]\n", os.Args[0])
		os.Exit(2)
	}

	text := os.Args[1]
	paneID, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid pane_id: %v\n", err)
		os.Exit(2)
	}

	// Optional wait_ms parameter
	waitMs := 0
	if len(os.Args) >= 4 {
		waitMs, _ = strconv.Atoi(os.Args[3])
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

	// Run GUI - blocks until user approves or rejects (and wait countdown completes)
	result := runApprovalGUI(text, paneID, shellType, waitMs)

	// Output minimal JSON to stdout (server handles output capture)
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

func runApprovalGUI(text string, paneID int, shellType string, waitMs int) interface{} {
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
	approved := false
	var approveTime time.Time

	var finalResult interface{}

	approveBtn := widget.NewButton(fmt.Sprintf("Approve [%s]", config.Active.Shortcuts.Approve), nil)
	editBtn := widget.NewButton(fmt.Sprintf("Show+Edit [%s]", config.Active.Shortcuts.Edit), nil)
	rejectBtn := widget.NewButton(fmt.Sprintf("Reject [%s]", config.Active.Shortcuts.Reject), nil)

	// Wait button: disabled initially, shows configured wait time
	waitBtnText := "0.0s"
	if waitMs > 0 {
		waitBtnText = fmt.Sprintf("%.1fs", float64(waitMs)/1000.0)
	}
	waitBtn := widget.NewButton(waitBtnText, nil)
	waitBtn.Disable()

	// Hint label for skip functionality (shown during countdown)
	waitHintLabel := widget.NewLabel("")
	waitHintLabel.Hide()

	approveBtn.OnTapped = func() {
		if edited {
			paneops.SendEnterToPane(paneID, shellType)
		} else {
			paneops.SendTextWithNewline(paneID, text, shellType)
		}

		approved = true
		approveTime = time.Now()
		finalResult = ApprovalResult{PaneID: paneID, ApprovedByUser: true}

		// Disable action buttons
		approveBtn.Disable()
		editBtn.Disable()
		rejectBtn.Disable()

		if waitMs > 0 {
			// Start countdown
			waitBtn.Enable()
			waitHintLabel.SetText("Click timer to skip waiting")
			waitHintLabel.Show()
			remaining := float64(waitMs) / 1000.0

			ticker := time.NewTicker(200 * time.Millisecond)
			done := make(chan struct{})

			go func() {
				for {
					select {
					case <-done:
						return
					case <-ticker.C:
						remaining -= 0.2
						if remaining <= 0 {
							remaining = 0
							ticker.Stop()
							close(done)
							go a.Quit()
							return
						}
						// Update button text
						remainingCopy := remaining
						waitBtn.SetText(fmt.Sprintf("%.1fs", remainingCopy))
					}
				}
			}()

			// Skip button handler
			waitBtn.OnTapped = func() {
				if !approved {
					return
				}
				ticker.Stop()
				select {
				case <-done:
				default:
					close(done)
				}
				a.Quit()
			}
		} else {
			// No wait - exit immediately
			a.Quit()
		}
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
	}, func(_ fyne.Shortcut) { approveBtn.OnTapped() })

	w.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyName(config.Active.Shortcuts.Reject),
		Modifier: 0,
	}, func(_ fyne.Shortcut) { rejectBtn.OnTapped() })

	w.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyName(config.Active.Shortcuts.Edit),
		Modifier: 0,
	}, func(_ fyne.Shortcut) { editBtn.OnTapped() })

	// Button row: action buttons left, wait button pushed right
	buttonRow := container.NewBorder(nil, nil, container.NewHBox(rejectBtn, editBtn, approveBtn), waitBtn)

	content := container.NewVBox(
		infoLabel,
		widget.NewSeparator(),
		widget.NewLabel("Text to send:"),
		textEntry,
		buttonRow,
		waitHintLabel,
	)

	w.SetContent(content)
	w.ShowAndRun()

	// Compute elapsed time after dialog closes (avoids race condition)
	if r, ok := finalResult.(ApprovalResult); ok && approved {
		r.TimeElapsedMs = time.Since(approveTime).Milliseconds()
		return r
	}
	if finalResult != nil {
		return finalResult
	}
	return RejectionResult{Error: "Dialog closed without action"}
}
