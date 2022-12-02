package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
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
	seen      []int
	client    api.RESTClient
	id        string
	seenmutex *sync.RWMutex
}

func newGistClient(client api.RESTClient, gistID string) *gistClient {
	return &gistClient{
		seen:      []int{},
		client:    client,
		id:        gistID,
		seenmutex: &sync.RWMutex{},
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
	perPage := 10
	c.seenmutex.RLock()
	page := (len(c.seen) / perPage) + 1
	c.seenmutex.RUnlock()
	u := fmt.Sprintf("gists/%s/comments?per_page=%d&page=%d", c.id, perPage, page)
	err := c.client.Get(u, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments: %w", err)
	}

	msgs := []string{}
	for _, comment := range resp {
		found := false
		c.seenmutex.RLock()
		for _, id := range c.seen {
			if comment.ID == id {
				found = true
				break
			}
		}
		c.seenmutex.RUnlock()
		if found {
			continue
		}
		switch comment.Body {
		case "LOLJOIN":
			msgs = append(msgs, "LOLJOIN "+comment.User.Login)
		case "LOLPART":
			msgs = append(msgs, "LOLPART "+comment.User.Login)
		default:
			msg := comment.Body + "\n"
			if !strings.HasPrefix(msg, "~") {
				msg = fmt.Sprintf("%s: %s", comment.User.Login, msg)
			}
			msgs = append(msgs, msg)
		}
		c.seenmutex.Lock()
		c.seen = append(c.seen, comment.ID)
		c.seenmutex.Unlock()
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
	nicklist := tview.NewTextView()

	cancel := make(chan struct{})

	rw := sync.RWMutex{}
	present := map[string]bool{}

	loadNewComments := func(msgs []string) {
		for _, m := range msgs {
			if strings.HasPrefix(m, "LOLJOIN") {
				split := strings.Split(m, " ")
				if len(split) > 1 {
					rw.Lock()
					present[split[1]] = true
					rw.Unlock()
				}
				msgView.Write([]byte(fmt.Sprintf("whoa %s has joined!\n", split[1])))
			} else if strings.HasPrefix(m, "LOLPART") {
				split := strings.Split(m, " ")
				if len(split) > 1 {
					rw.Lock()
					present[split[1]] = false
					rw.Unlock()
				}
				msgView.Write([]byte(fmt.Sprintf("aw, %s left ;_;\n", split[1])))
			} else {
				msgView.Write([]byte(m))
			}
		}
		presentTxt := ""
		rw.RLock()
		for k, v := range present {
			if v {
				presentTxt += k + "\n"
			}
		}
		rw.RUnlock()
		nicklist.SetText(presentTxt)
		msgView.ScrollToEnd()
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
			gc.AddComment("\n" + banner)
		}
		txt := input.GetText()
		if strings.HasPrefix(txt, "/") {
			split := strings.SplitN(txt, " ", 2)
			switch split[0] {
			case "/help":
				lines := []string{
					"system:",
					"/quit (<message>)",
					"  leave chat with an optional part message",
					"/invite <username>",
					"  invite a github user to the chat. they'll get a notification on github.",
					"/banner <msg>",
					"  render an ascii banner",
					"/banner-font <font> <msg>",
					"  render an ascii banner with the chosen font. try script or shadow.",
				}
				for _, l := range lines {
					msgView.Write([]byte(l + "\n"))
				}
			case "/quit":
				quitMsg := ""
				if len(split) > 1 && split[1] != "" {
					quitMsg = fmt.Sprintf(" (%s)", split[1])
				}

				gc.AddComment(fmt.Sprintf("~ vilmibm quit%s\n", quitMsg))
				close(cancel)
				app.Stop()
			case "/banner":
				banner("standard", split[1])
			case "/banner-font":
				sp := strings.SplitN(split[1], " ", 2)
				banner(sp[0], sp[1])
			case "/invite":
				// TODO support stripping leading @ since i keep doing it
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

	innerFlex := tview.NewFlex()
	innerFlex.SetDirection(tview.FlexColumn)
	innerFlex.AddItem(msgView, 0, 5, false)
	innerFlex.AddItem(nicklist, 0, 1, false)

	outerFlex := tview.NewFlex()
	outerFlex.SetDirection(tview.FlexRow)
	outerFlex.AddItem(innerFlex, 0, 10, false)
	outerFlex.AddItem(input, 1, 1, true)

	app.SetRoot(outerFlex, true)

	gc.AddComment("LOLJOIN")
	defer gc.AddComment("LOLPART")

	go func(c chan struct{}) {
		for {
			select {
			case <-c:
				break
			default:
				msgs, err := gc.GetNewComments()
				if err != nil {
					msgView.Write([]byte(fmt.Sprintf("system error: %s\n", err.Error())))
					app.ForceDraw()
					continue
				}
				app.ForceDraw()

				loadNewComments(msgs)

				time.Sleep(time.Second * 4)
			}
		}
	}(cancel)

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
	} else if len(args) == 1 {
		// TODO support check if it ever works
		opts.GistID = args[0]
	} else {
		return errors.New("expected 0 or 1 arguments")
	}

	return joinChat(*opts)
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
