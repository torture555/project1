package main

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	//"fmt"
	"errors"
	"strconv"
	//"reflect"
	"bufio"
	"github.com/gorilla/websocket"
)

type file_ready_to_send struct {
	Filename string          `json: "filename"`
	Content  []*file_content `json: "content: "`
}

type file_content struct {
	Mark  float64 `json: "mark"`
	Value float64 `json: "value"`
}

var upgrader = websocket.Upgrader{}
var todoList []string
var filelist = []string{}

func getCmd(input string) string {
	inputArr := strings.Split(input, " ")
	return inputArr[0]
}

func getMessage(input string) string {
	inputArr := strings.Split(input, " ")
	var result string
	for i := 1; i < len(inputArr); i++ {
		result += inputArr[i]
	}
	return result
}

func updateTodoList(input string) {
	tmpList := todoList
	todoList = []string{}
	for _, val := range tmpList {
		if val == input {
			continue
		}
		todoList = append(todoList, val)
	}
}

func main() {
	mux := http.NewServeMux()
	filelistfull := false
	mux.HandleFunc("/todo", func(w http.ResponseWriter, r *http.Request) {

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade failed: ", err)
			return
		}
		defer conn.Close()

		for {
			mt, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("read failed:", err)
				break
			}
			input := string(message)
			cmd := getCmd(input)
			msg := getMessage(input)
			if cmd == "add" {
				todoList = append(todoList, msg)
			} else if cmd == "done" {
				updateTodoList(msg)
			} else if cmd == "UpdateFileList" {
				UpdateFileList(conn, filelistfull)
				filelistfull = true
			} else if strings.HasSuffix(cmd, "DeleteFile:") == true {
				DeleteFile(strings.ReplaceAll(input, "DeleteFile: ", ""))
				UpdateFileList(conn, filelistfull)
			} else if cmd == "AddFile:" {
				err = AddFileInDir(strings.ReplaceAll(input, "AddFile: ", ""))
				if err != nil {
					conn.WriteMessage(mt, []byte("ErrorAddFile: Не удалось загрузить файл!"))
				}
				UpdateFileList(conn, filelistfull)
			} else if cmd == "GetChartInfo:" {
				info, err := GetChartInfo(strings.ReplaceAll(input, "GetChartInfo: ", ""))
				if err != nil {
					conn.WriteMessage(mt, []byte("ErrGetChartInfo: Не удалось получить данные для вывода графика!"))
				} else {
					inf := &info
					json_msg, err := json.Marshal(inf)
					if err != nil {
						break
					}
					conn.WriteMessage(mt, json_msg)
				}
			}
			output := "Current Todos: \n"
			for _, todo := range todoList {
				output += "\n - " + todo + "\n"
			}
			output += "\n----------------------------------------"
			message = []byte(output)
			err = conn.WriteMessage(mt, message)
			if err != nil {
				log.Println("write failed:", err)
				break
			}
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "public/index.html")
	})

	mux.ListenAndServe("192.168.0.1:8080", mux)
}

func UpdateFileList(conn *websocket.Conn, clearlist bool) {
	filelist = nil
	files, err := os.ReadDir("./datafiles")
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".txt") == true {
			filelist = append(filelist, strings.ReplaceAll(file.Name(), ".txt", ""))
		}
	}
	if len(filelist) < 1 {
		filelist = nil
	}

	if clearlist == true {
		err = conn.WriteMessage(1, []byte("clearlist: "))
		if err != nil {
			log.Fatal(err)
		}
	}

	for i := range filelist {
		err = conn.WriteMessage(1, []byte("file: "+filelist[i]))
		if err != nil {
			log.Println("Can't read file: ", filelist[i], err)
			break
		}
	}
}

func DeleteFile(filename string) {
	filename = filename + ".txt"
	err := os.Remove("./datafiles/" + filename)
	if err != nil {
		log.Fatal("Не удален файл: ", err)
	}
}

func AddFileInDir(input string) error {
	name := strings.Split(input, ",")[0]
	f, err := os.Create("datafiles/" + name)
	if err != nil {
		log.Println(err)
	}
	_, err = f.Write([]byte(strings.ReplaceAll(input, name+",", "")))
	if err != nil {
		log.Println(err)
	}
	err = f.Close()
	if err != nil {
		log.Println(err)
	}
	return err
}

func GetChartInfo(filename string) (interface{}, error) {
	f, err := os.Open("datafiles/" + filename + ".txt")
	if err != nil {
		log.Println(err)
		return nil, err
	}

	reader := bufio.NewScanner(f)
	var lines []string
	for reader.Scan() {
		lines = append(lines, reader.Text())
	}

	var result file_ready_to_send

	result.Filename = filename

	for _, line := range lines {
		if strings.HasSuffix(line, "#") == true {
			continue
		}
		i := strings.Split(line, " ")
		if len(i) != 2 {
			continue
		}
		var j file_content
		for k, values := range i {
			l := strings.Split(values, "e")
			intenger, err := strconv.ParseFloat(l[0], 64)
			if err != nil {
				break
			}
			power, err := strconv.ParseFloat(l[1], 64)
			if err != nil {
				break
			}
			if k == 0 {
				j.Mark = intenger * math.Pow(10, power)
			} else {
				j.Value = intenger * math.Pow(10, power)
			}
		}
		result.Content = append(result.Content, &j)
	}
	if len(result.Content) == 0 {
		return nil, errors.New("Не доступен результат, возможно файл пустой")
	}
	f.Close()
	return result, nil
}
