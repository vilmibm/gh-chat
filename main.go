package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/cli/go-gh"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

/*
	i have accepted the limitation that this command creates a room that then
	has to be transmitted out-of-band to other users. the upshot of this is
	it's easier to have more than 2 people in a chat. downside is how it's
	falsifiable, but i am ok with that for a stupid hackathon.
*/

func main() {
	client, err := gh.RESTClient(nil)
	if err != nil {
		panic(err)
	}
	response := struct{ Login string }{}
	err = client.Get("user", &response)
	if err != nil {
		panic(err)
	}
	username := response.Login

	var gistOwner string
	var gistID string
	var enter bool
	if len(os.Args) == 1 {
		gistOwner = username
		// TODO actually create gist
		gistID = "b6f867cbdd5dcb3e08fca1323fae4db8"
		fmt.Printf("created chat room. tell others to run `gh chat %s/%s`\n",
			gistOwner, gistID)
		survey.AskOne(&survey.Confirm{
			Message: "continue into chat room?",
			Default: true,
		}, &enter)
		defer cleanupGist(gistOwner, gistID)
	} else {
		split := strings.Split(os.Args[1], "/")
		gistOwner = split[0]
		gistID = split[1]
	}

	cloneDir := path.Join(os.TempDir(), gistID)

	if !enter {
		return
	}

	gistURL := fmt.Sprintf("https://gist.github.com/%s/%s", gistOwner, gistID)
	cloneArgs := []string{"clone", gistURL, cloneDir}
	cloneCmd := exec.Command("git", cloneArgs...)
	cloneCmd.Stderr = os.Stderr
	err = cloneCmd.Run()
	if err != nil {
		panic(err)
	}
	defer cleanupClone(cloneDir)

	/*
		msgBuffs := map[string]io.Reader{
			gistOwner: ownerBuff,
		}
	*/

	app := tview.NewApplication()

	mw := newMsgWriter(cloneDir, username)

	msgView := tview.NewTextView()
	input := tview.NewInputField()
	input.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			return
		}
		mw.Write(input.GetText())
		//_, _ = msgView.Write([]byte(input.GetText()))
	})

	flex := tview.NewFlex()
	flex.SetDirection(tview.FlexRow)
	flex.AddItem(msgView, 0, 10, false)
	flex.AddItem(input, 1, 1, true)

	app.SetRoot(flex, true)

	lastCheck := time.Now().Unix()

	go func() {
		for true {
			pullCmd := exec.Command("git", "pull", "--rebase")
			pullCmd.Dir = cloneDir
			err = pullCmd.Run()
			if err != nil {
				panic(err)
			}
			files, err := os.ReadDir(cloneDir)
			if err != nil {
				panic(err)
			}

			for _, f := range files {
				if strings.HasSuffix(f.Name(), ".chat") {
					fullPath := path.Join(cloneDir, f.Name())
					chatContents, err := os.ReadFile(fullPath)
					if err != nil {
						panic(err)
					}
					lines := strings.Split(string(chatContents), "\n")
					for _, l := range lines {
						split := strings.SplitN(l, " ", 2)
						if len(split) != 2 {
							continue
						}
						msg := split[1]
						when, err := strconv.Atoi(split[0])
						if err != nil {
							panic(err)
						}
						if int64(when) > lastCheck {
							un := strings.TrimSuffix(f.Name(), ".chat")
							msgView.Write([]byte(fmt.Sprintf("%s: %s\n", un, msg)))
						}
					}
				}
			}
			lastCheck = time.Now().Unix()
			time.Sleep(time.Second * 5)
		}
	}()

	if err := app.Run(); err != nil {
		panic(err)
	}
}

type msgWriter struct {
	Username string
	CloneDir string
}

func newMsgWriter(cloneDir, username string) *msgWriter {
	cpath := path.Join(cloneDir, username+".chat")
	_, err := os.Create(cpath)
	if err != nil {
		panic(err)
	}

	addCmd := exec.Command("git", "add", cpath)
	addCmd.Dir = cloneDir
	addCmd.Run()

	commitCmd := exec.Command("git", "commit", "-a", "-m", "TODO")
	commitCmd.Dir = cloneDir
	commitCmd.Run()

	pushCmd := exec.Command("git", "push")
	pushCmd.Dir = cloneDir
	pushCmd.Run()

	return &msgWriter{
		CloneDir: cloneDir,
		Username: username,
	}
}

func (w *msgWriter) Write(text string) {
	buff, err := os.Open(path.Join(w.CloneDir, w.Username+".chat"))
	if err != nil {
		panic(err)
	}
	defer buff.Close()
	msg := fmt.Sprintf("%d %s\n", time.Now().Unix(), text)
	buff.Write([]byte(msg))
	commitCmd := exec.Command("git", "commit", "-a", "-m", "TODO")
	commitCmd.Dir = w.CloneDir
	buf := &bytes.Buffer{}
	commitCmd.Stderr = buf
	commitCmd.Stdout = buf
	err = commitCmd.Run()
	if err != nil {
		panic(buf.String())
	}

	pullCmd := exec.Command("git", "pull", "--rebase")
	pullCmd.Dir = w.CloneDir
	err = pullCmd.Run()
	if err != nil {
		panic(err)
	}

	pushCmd := exec.Command("git", "push")
	pushCmd.Dir = w.CloneDir
	err = pushCmd.Run()
	if err != nil {
		panic(err)
	}
}

func cleanupClone(cloneDir string) {
	os.RemoveAll(cloneDir)
}

func cleanupGist(gistOwner, gistID string) {
	// TODO delete gist
}
