package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type ChatGPTRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type ChatGPTResponse struct {
	Id      string   `json:"id"`
	Object  string   `json:"object"`
	Created string   `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index         int     `json:"index"`
	Message       Message `json:"message"`
	Finish_reason string  `json:"finish_reason"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalToken       int `json:"total_tokens"`
}
type SaveFile struct {
	Name     string    `json:"name"`
	Messages []Message `json:"messages"`
}

func escapeAnsi(s string) string {
	return strings.ReplaceAll(s, "\\033", "\033")
}

func setHeaders(req *http.Request, key string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
func saveChat(chat []Message, name string) {

	filename := fmt.Sprintf("chat_%d.json", time.Now().Unix())
	nf, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)

	defer nf.Close()
	check(err)
	nw := bufio.NewWriter(nf)
	fileData := SaveFile{Name: name, Messages: chat}
	jsonFileData, err := json.Marshal(fileData)
	_, err = nw.Write(jsonFileData)
	check(err)
	nw.Flush()
	os.Exit(1)
}
func getChats() []SaveFile {
	var files []SaveFile

	c, err := os.ReadDir(".")
	check(err)

	for _, entry := range c {
		f, err := os.OpenFile(entry.Name(), os.O_RDONLY, 0644)
		if filepath.Ext(entry.Name()) == ".json" {

			check(err)
			defer f.Close()

			jsonData, err := io.ReadAll(f)
			check(err)

			var savedFiled SaveFile

			err = json.Unmarshal(jsonData, &savedFiled)
			check(err)

			files = append(files, savedFiled)
		}

	}

	return files

}
func getChatName(key string, name string, client *http.Client, prompt string) string {

	chatNamePromptContent := fmt.Sprintf("Give me a simple really short sentence like 3 to 4 word to describe the following prompt : %s. I just want the short sentence nothing esle", prompt)

	chatNamePrompt := Message{Role: "user", Content: chatNamePromptContent}
	chatNameRequestBody := ChatGPTRequest{
		Model:    "gpt-4",
		Messages: []Message{chatNamePrompt},
	}
	jsonBody, err := json.Marshal(chatNameRequestBody)
	check(err)
	chatNameReq, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonBody))
	check(err)
	setHeaders(chatNameReq, key)
	res, err := client.Do(chatNameReq)

	check(err)
	defer res.Body.Close()

	var resData ChatGPTResponse
	json.NewDecoder(res.Body).Decode(&resData)

	return resData.Choices[0].Message.Content

}

func main() {
	p := fmt.Println
	p()

	f, err := os.OpenFile("clagent.conf", os.O_RDWR|os.O_CREATE, 0644)
	check(err)
	defer f.Close()

	scanner := bufio.NewScanner(f)
	keymap := make(map[string]string)
	for scanner.Scan() {
		l := strings.Split(scanner.Text(), "=")
		keymap[l[0]] = l[1]
	}

	prevChats := getChats()
	var selectedChat uint16

	if len(prevChats) > 0 {

		p("Select a chat or create a new one")
		for i, file := range prevChats {
			fmt.Printf("%d. %s\n", i+1, file.Name)
		}
		p("0. New chat")
		p()
		fmt.Print("What's your choice : ")
		var err error
		fmt.Scanf("%d", &selectedChat)
		check(err)
	}

	confWriter := bufio.NewWriter(f)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	var messages []Message
	if selectedChat != 0 {

		messages = prevChats[selectedChat-1].Messages
	} else {

		messages = []Message{{Role: "system", Content: "You have to answere with ANSI color or some bold on some words to emphasize your responses, i really don't want you to escape the ANSI"}}
	}
	var ai int = 1
	switch ai {
	case 1:

		p("\033[1;35mHey i'm GPT How can i help you today\033[0m")
		p()
		key := keymap["OPENAI_API_KEY"]
		chatName := ""
		go func() {
			<-sigs
			if chatName != "" {

				saveChat(messages, chatName)
			}
		}()
		if key == "" {
			p("You didn't provide your OPENAI_API_KEY yet")
			fmt.Print("\nEnter it here : ")
			fmt.Scanf("%s", &key)

			confWriter.WriteString("OPENAI_API_KEY=" + key + "\n")

			confWriter.Flush()
			p("Your apikey has been written")
		}
		reader := bufio.NewReader(os.Stdin)
		for {

			client := &http.Client{}
			p("\033[1;36mYou :\033[0m")
			p()
			prompt, err := reader.ReadString('\n')
			if chatName == "" {
				chatName = getChatName(key, chatName, client, prompt)
			}
			p()
			p()
			messages = append(messages, Message{Role: "user", Content: prompt})

			requestBody := ChatGPTRequest{
				Model:    "gpt-4",
				Messages: messages,
			}

			jsonData, err := json.Marshal(requestBody)
			check(err)

			req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
			setHeaders(req, key)

			resp, err := client.Do(req)
			check(err)
			defer resp.Body.Close()

			var result ChatGPTResponse
			json.NewDecoder(resp.Body).Decode(&result)

			p("\033[1;36mResponse from GPT\033[0m")
			p()
			respData := escapeAnsi(result.Choices[0].Message.Content)

			messages = append(messages, Message{Role: "assistant", Content: respData})
			fmt.Println(respData)
			p()
			p()
		}

	case 2:
		p("You choose Gemini")
		key := os.Getenv("GEMINI_API_KEY")
		if key == "" {
			p("You didn't provid you Gemini api key ")
			fmt.Print("Enter your key : ")
			fmt.Scanf("%s", &key)
			p()
			os.Setenv("GEMINI_API_KEY", key)
			p("Perfect ! your key is now set in your env")
		}
		fmt.Println(key)

	case 3:
		p("You choose Claude")
		key := os.Getenv("CLAUDE_API_KEY")
		if key == "" {
			p("You didn't provid you Claude api key ")
			fmt.Print("Enter your key : ")
			fmt.Scanf("%s", &key)
			p()
			os.Setenv("CLAUDE_API_KEY", key)
			p("Perfect ! your key is now set in your env")
		}
		fmt.Println(key)
	}
}
