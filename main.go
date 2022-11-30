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
	"github.com/cli/go-gh/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TODO add interrupt code to gh in a branch
type gistFile struct {
	Content string `json:"content"`
}

type gistClient struct {
	client api.RESTClient
	id     string
}

func newGistClient(client api.RESTClient, gistID string) *gistClient {
	return &gistClient{
		client: client,
		id:     gistID,
	}
}

func (c *gistClient) AddComment(text string) error {
	args := &struct {
		GistID string `json:"gist_id"`
		Body   string `json:"body"`
	}{
		GistID: c.id,
		Body:   text,
	}
	jargs, err := json.Marshal(args)
	if err != nil {
		return err
	}

	return c.client.Post(fmt.Sprintf("gists/%s/comments", c.id), bytes.NewBuffer(jargs), nil)
}

func _main(args []string) error {
	client, err := gh.RESTClient(nil)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	response := struct{ Login string }{}
	err = client.Get("user", &response)
	if err != nil {
		panic(err)
	}
	currentUsername := response.Login

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
		gistID = args[0]
		enter = true
	}

	if !enter {
		return nil
	}

	gc := newGistClient(client, gistID)

	app := tview.NewApplication()

	msgView := tview.NewTextView()
	input := tview.NewInputField()
	input.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			return
		}
		defer input.SetText("")
		txt := input.GetText()
		if strings.HasPrefix(txt, "/") {
			split := strings.SplitN(txt, " ", 2)
			switch split[0] {
			case "/quit":
				app.Stop()
			case "/invite":
				err = gc.AddComment(
					fmt.Sprintf("~ hey @%s come chat ^_^ `gh ext install vilmibm/gh-chat && gh chat %s`", split[1], gistID))
				if err != nil {
					msgView.Write([]byte(fmt.Sprintf("system error: %s\n", err.Error())))
				}
			case "/me":
				err = gc.AddComment(fmt.Sprintf("~ %s %s", currentUsername, split[1]))
				if err != nil {
					msgView.Write([]byte(fmt.Sprintf("system error: %s\n", err.Error())))
				}
			}
			return
		}
		err = gc.AddComment(txt)
		if err != nil {
			msgView.Write([]byte(fmt.Sprintf("system error: %s\n", err.Error())))
		}
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
				msg := c.Body + "\n"
				if !strings.HasPrefix(msg, "~") {
					msg = fmt.Sprintf("%s: %s", c.User.Login, msg)
				}
				msgView.Write([]byte(msg))
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
