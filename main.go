package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/cli/go-gh"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TODO add interrupt code to gh in a branch
// TODO consider protocol approach so things like invites can be handled accordingly
type gistFile struct {
	Content string `json:"content"`
}

func _main(args []string) error {
	client, err := gh.RESTClient(nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	files := map[string]gistFile{}
	files["chat"] = gistFile{
		Content: "lol hey",
	}

	var gistID string
	var enter bool
	if len(args) == 0 {
		body, err := json.Marshal(struct {
			Files  map[string]gistFile `json:"files"`
			Public bool                `json:"public"`
		}{
			Files:  files,
			Public: false,
		})
		if err != nil {
			return fmt.Errorf("could not marshal gist data: %w", err)
		}
		resp := struct {
			ID string `json:"id"`
		}{}
		err = client.Post("gists", bytes.NewReader(body), &resp)
		if err != nil {
			return fmt.Errorf("failed to create gist: %w", err)
		}
		gistID = resp.ID
		defer cleanupGist(gistID)

		fmt.Printf("created chat room. others can join with `gh chat %s`\n", gistID)
		err = survey.AskOne(&survey.Confirm{
			Message: "continue into chat room?",
			Default: true,
		}, &enter)
		if err != nil {
			return fmt.Errorf("could not prompt: %w", err)
		}
	} else {
		split := strings.Split(args[0], "/")
		gistID = split[0]
		enter = true
	}

	if !enter {
		return nil
	}

	app := tview.NewApplication()

	msgView := tview.NewTextView()
	input := tview.NewInputField()
	input.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			return
		}
		txt := input.GetText()
		if strings.HasPrefix(txt, "/") {
			// TODO handle / commands
			return
		}
		args := &struct {
			GistID string `json:"gist_id"`
			Body   string `json:"body"`
		}{
			GistID: gistID,
			Body:   input.GetText(),
		}
		response := &struct{}{}
		jargs, err := json.Marshal(args)
		if err != nil {
			msgView.Write([]byte(fmt.Sprintf("system error: %s\n", err.Error())))
		}

		err = client.Post(fmt.Sprintf("gists/%s/comments", gistID), bytes.NewBuffer(jargs), &response)
		if err != nil {
			msgView.Write([]byte(fmt.Sprintf("system error: %s\n", err.Error())))
		}
		input.SetText("")
	})

	flex := tview.NewFlex()
	flex.SetDirection(tview.FlexRow)
	flex.AddItem(msgView, 0, 10, false)
	flex.AddItem(input, 1, 1, true)

	app.SetRoot(flex, true)

	go func() {
		seen := []int{}

		for true {
			resp := []struct {
				Body string
				ID   int `json:"id"`
				User struct {
					Login string
				}
			}{}
			err := client.Get(fmt.Sprintf("gists/%s/comments", gistID), &resp)
			if err != nil {
				msgView.Write([]byte(fmt.Sprintf("system error: %s\n", err.Error())))
				app.ForceDraw()
				continue
			}

			for _, c := range resp {
				found := false
				for _, id := range seen {
					if c.ID == id {
						found = true
						break
					}
				}
				if found {
					continue
				}
				msgView.Write([]byte(fmt.Sprintf("%s: %s\n", c.User.Login, c.Body)))
				app.ForceDraw()
				seen = append(seen, c.ID)
			}

			time.Sleep(time.Second * 4)
		}
	}()

	if err := app.Run(); err != nil {
		return fmt.Errorf("tview error: %w", err)
	}

	return nil
}

func main() {
	args := []string{}
	if len(os.Args) > 1 {
		args = os.Args[1:]
	}
	err := _main(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
}

func cleanupGist(gistID string) {
	client, err := gh.RESTClient(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create client during cleanup: %s", err.Error())
	}

	err = client.Delete(fmt.Sprintf("gists/%s", gistID), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to cleanup gist %s: %s", gistID, err.Error())
	}
}
