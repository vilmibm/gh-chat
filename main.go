package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/cli/go-gh"
	"github.com/cli/go-gh/pkg/api"
	"github.com/gdamore/tcell/v2"
	"github.com/lukesampson/figlet/figletlib"
	"github.com/rivo/tview"
)

//go:embed fonts/*
var fonts embed.FS

type gistFile struct {
	Content string `json:"content"`
}

type gistComment struct {
	Body string
	ID   int `json:"id"`
	User struct {
		Login string
	}
}

type gistClient struct {
	seen   []int
	client api.RESTClient
	id     string
}

func newGistClient(client api.RESTClient, gistID string) *gistClient {
	return &gistClient{
		seen:   []int{},
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

func (c *gistClient) GetNewComments() ([]string, error) {
	resp := []gistComment{}
	err := c.client.Get(fmt.Sprintf("gists/%s/comments", c.id), &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments: %w", err)
	}

	msgs := []string{}
	for _, comment := range resp {
		found := false
		for _, id := range c.seen {
			if comment.ID == id {
				found = true
				break
			}
		}
		if found {
			continue
		}
		msg := comment.Body + "\n"
		if !strings.HasPrefix(msg, "~") {
			msg = fmt.Sprintf("%s: %s", comment.User.Login, msg)
		}
		msgs = append(msgs, msg)
		c.seen = append(c.seen, comment.ID)
	}
	return msgs, nil
}

type ChatOpts struct {
	Client   api.RESTClient
	Username string
	GistID   string
}

func createChat(opts ChatOpts) (string, error) {
	gistFilename := fmt.Sprintf("%s's chat room in gh", opts.Username)
	files := map[string]gistFile{}
	files[gistFilename] = gistFile{
		Content: "i'm using https://github.com/vilmibm/gh-chat to chat in the terminal.",
	}
	body, err := json.Marshal(struct {
		Files  map[string]gistFile `json:"files"`
		Public bool                `json:"public"`
	}{
		Files:  files,
		Public: false,
	})
	if err != nil {
		return "", fmt.Errorf("could not marshal gist data: %w", err)
	}
	resp := struct {
		ID string `json:"id"`
	}{}
	err = opts.Client.Post("gists", bytes.NewReader(body), &resp)
	if err != nil {
		return "", fmt.Errorf("failed to create gist: %w", err)
	}
	return resp.ID, nil
}

func joinChat(opts ChatOpts) error {
	gistID := opts.GistID
	gc := newGistClient(opts.Client, gistID)

	app := tview.NewApplication()

	msgView := tview.NewTextView()

	loadNewComments := func(msgs []string) {
		for _, m := range msgs {
			msgView.Write([]byte(m))
		}
		app.ForceDraw()
	}
	input := tview.NewInputField()
	input.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			return
		}
		defer input.SetText("")
		banner := func(fontFile string, text string) {
			data, err := fonts.ReadFile("fonts/" + fontFile + ".flf")
			if err != nil {
				msgView.Write([]byte(fmt.Sprintf("system error: %s\n", err.Error())))
				return
			}

			f, err := figletlib.ReadFontFromBytes(data)
			if err != nil {
				msgView.Write([]byte(fmt.Sprintf("system error: %s\n", err.Error())))
				return
			}

			_, _, width, _ := msgView.GetRect()
			banner := figletlib.SprintMsg(text, f, width, f.Settings(), "left")
			msgView.Write([]byte(opts.Username + ":\n"))
			msgView.Write([]byte(banner))
			msgView.Write([]byte("\n"))
		}
		txt := input.GetText()
		if strings.HasPrefix(txt, "/") {
			split := strings.SplitN(txt, " ", 2)
			switch split[0] {
			case "/quit":
				app.Stop()
			case "/banner":
				banner("standard", split[1])
			case "/banner-font":
				sp := strings.SplitN(split[1], " ", 2)
				banner(sp[0], sp[1])
			case "/invite":
				err := gc.AddComment(
					fmt.Sprintf("~ hey @%s come chat ^_^ `gh ext install vilmibm/gh-chat && gh chat %s`", split[1], gistID))
				if err != nil {
					msgView.Write([]byte(fmt.Sprintf("system error: %s\n", err.Error())))
				}
				msgs, err := gc.GetNewComments()
				if err != nil {
					msgView.Write([]byte(fmt.Sprintf("system error: %s\n", err.Error())))
				}
				go loadNewComments(msgs)
			case "/me":
				err := gc.AddComment(fmt.Sprintf("~ %s %s", opts.Username, split[1]))
				if err != nil {
					msgView.Write([]byte(fmt.Sprintf("system error: %s\n", err.Error())))
				}
				msgs, err := gc.GetNewComments()
				if err != nil {
					msgView.Write([]byte(fmt.Sprintf("system error: %s\n", err.Error())))
				}
				go loadNewComments(msgs)
			}
			return
		}
		err := gc.AddComment(txt)
		if err != nil {
			msgView.Write([]byte(fmt.Sprintf("system error: %s\n", err.Error())))
		}
		msgs, err := gc.GetNewComments()
		if err != nil {
			msgView.Write([]byte(fmt.Sprintf("system error: %s\n", err.Error())))
		}
		go loadNewComments(msgs)
	})

	flex := tview.NewFlex()
	flex.SetDirection(tview.FlexRow)
	flex.AddItem(msgView, 0, 10, false)
	flex.AddItem(input, 1, 1, true)

	app.SetRoot(flex, true)

	go func() {
		for true {
			msgs, err := gc.GetNewComments()
			if err != nil {
				msgView.Write([]byte(fmt.Sprintf("system error: %s\n", err.Error())))
				app.ForceDraw()
				continue
			}

			loadNewComments(msgs)

			time.Sleep(time.Second * 4)
		}
	}()

	if err := app.Run(); err != nil {
		return fmt.Errorf("tview error: %w", err)
	}
	return nil
}

func checkForChat(opts ChatOpts) error {
	// TODO
	// the purpose of this function is to look for a recent notification inviting
	// opts.Username to a chat and then reporting on that fact with instructions
	// on how to join. This could be called from a .bashrc to alert a user that a
	// chat is waiting for them.

	// oooof gist notifications are filtered out of the notifications API :( :(

	/*

		result := []struct {
			Subject struct {
				Title string
			}
		}{}

		err := opts.Client.Get("notifications", &result)
		if err != nil {
			return fmt.Errorf("failed to get notifications: %w", err)
		}

		for _, n := range result {

		}
	*/

	return errors.New("tragically, the github notifications api filters out gist notifications")
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

	opts := &ChatOpts{
		Client:   client,
		Username: currentUsername,
	}

	if len(args) == 0 {
		gistID, err := createChat(*opts)
		if err != nil {
			return err
		}
		opts.GistID = gistID

		defer cleanupGist(gistID)

		enter := false
		fmt.Printf("created chat room. others can join with `gh chat %s`\n", gistID)
		err = survey.AskOne(&survey.Confirm{
			Message: "continue into chat room?",
			Default: true,
		}, &enter)
		if err != nil {
			return fmt.Errorf("could not prompt: %w", err)
		}
		if !enter {
			return nil
		}

		return joinChat(*opts)
	} else if len(args) == 1 {
		switch args[0] {
		case "check":
			return checkForChat(*opts)
		default:
			opts.GistID = args[0]
			return joinChat(*opts)
		}
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
