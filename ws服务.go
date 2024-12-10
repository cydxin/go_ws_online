package main

import (
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	//writeWait      = 100 * time.Second
	pongWait = 100 * time.Second
	//pingPeriod     = (pongWait * 900) / 10
	maxMessageSize = 512
)

// 存储所有 WebSocket 连接的 map 和一个保护映射的锁
// var clients = make(map[int]*websocket.Conn)
//var mu sync.Mutex

type Client struct {
	ClientID string          `json:"id"`
	Conn     *websocket.Conn `json:"-"`
	MsgChan  chan []byte     `json:"-"`
	Manger   bool            `json:"man"`
	//msgChan chan []byte     `json:"-"`
}
type adminClient struct {
	Conn      *websocket.Conn `json:"-"`
	AllClient *AllClient      `json:"-"`
}

type AllClient struct {
	Clients map[string]*Client `json:"clients"`
	Lock    sync.Mutex         `json:"-"`
}

var upgrade = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var allClient = adminClient{
	AllClient: &AllClient{
		Clients: make(map[string]*Client),
	},
}

func (c *Client) readMsg() {
	defer func() { // 协程结束后会执行的操作
		err := c.Conn.Close()
		// 删除连接
		allClient.AllClient.Lock.Lock()
		delete(allClient.AllClient.Clients, c.ClientID)
		allClient.AllClient.Lock.Unlock()
		if err != nil {
			return
		}
	}()
	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetPongHandler(func(string) error {
		err := c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		if err != nil {
			return err
		}
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		var messageD []string
		err = sonic.Unmarshal(message, &messageD)
		if err != nil {
			log.Fatal(err, "解析消息失败", string(message))
		}
		switch messageD[0] {
		case "admin8741aw7fe4gtgb":
			c.Manger = true
			sendChatListToAdmin(c.Conn)
		case "msg":
			for _, i := range allClient.AllClient.Clients {
				if i.Manger == true {
					i.MsgChan <- []byte(messageD[1])
				}
			}
		case "toUser":
			// 管理员连接
			if c.Manger == true {
				// 断言第二位是string
				if cli, ok := allClient.AllClient.Clients[messageD[1]]; ok {
					cli.MsgChan <- []byte(messageD[2])
				}
			}
		default:
			//log.Println(messageD, "default")
		}
	}
}
func (c *Client) writeMsg() {
	for {
		msg := <-c.MsgChan
		mess := fmt.Sprintf(`["message","%v"]`, string(msg))
		fmt.Println("writeMsg", mess)
		err := c.Conn.WriteMessage(websocket.TextMessage, []byte(mess))
		if err != nil {
			log.Fatalf("发送消息失败 : %v", err)
		}
		//select {
		//case msg := <-c.MsgChan:
		//	err := c.Conn.WriteMessage(websocket.TextMessage, msg)
		//	if err != nil {
		//		log.Fatalf("发送消息失败 : %v", err)
		//	}
		//}
	}
}
func (c *Client) sendMsg(msg []byte) {
	//err := c.Conn.WriteMessage(websocket.TextMessage, append([]byte{'4', '2'}, msg...))
	err := c.Conn.WriteMessage(websocket.TextMessage, msg)
	if err != nil {
		return
	}
}

func handleConnectionsQA(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")
	list := []map[string]string{
		{"question": "1+1=?", "answer": "2"},
		{"question": "1+1=?", "answer": "2"},
		{"question": "1+1=?", "answer": "2"},
		{"question": "1+1=?", "answer": "2"},
		{"question": "1+1=?", "answer": "2"},
	}
	marshal, err := sonic.Marshal(list)
	if err != nil {
		return
	}
	_, err = w.Write(marshal)
	if err != nil {
		log.Fatal(err)
	}
}

// 处理文件上传
func handleConnectionsUpload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")
	// 读取 文件
	file, m, err := r.FormFile("file")
	if err != nil {
		log.Println("读取文件失败", err)
	}

	log.Println(file)
	log.Println(m.Filename)
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	// 升级 HTTP 请求为 WebSocket 连接
	ws, err := upgrade.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	sid := r.Header.Get("Sec-Websocket-Key")

	userClient := Client{
		ClientID: sid,
		Conn:     ws,
		MsgChan:  make(chan []byte), // 初始化 MsgChan
		Manger:   false,
	}
	// 假设用一个自增的 ID 作为键来存储连接
	allClient.AllClient.Lock.Lock()
	allClient.AllClient.Clients[sid] = &userClient
	//allClient.AllClient.Clients = append(allClient.AllClient.Clients, &userClient)
	allClient.AllClient.Lock.Unlock()

	go userClient.readMsg()
	go userClient.writeMsg()
	// 处理接收和转发消息
	if msg, err := sonic.Marshal(map[string]interface{}{
		"pingInterval": 25000,
		"pingTimeout":  5000,
		"sid":          sid, //不相关没事
		"upgrades":     make([]int, 0),
	}); err != nil {
		log.Fatal("初始化的client错误 : %v", err)
	} else {
		//socket.io风格的初始数据
		userClient.sendMsg(msg)
		//userClient.sendMsg(append([]byte{'0'}, msg...))
		//userClient.sendMsg(append([]byte{'4', '0'}))
	}
}

// 向管理员发送当前所有客户端的聊天信息
func sendChatListToAdmin(ws *websocket.Conn) {
	chanList, _ := sonic.Marshal([]interface{}{
		"list",
		allClient.AllClient.Clients,
	})
	err := ws.WriteMessage(websocket.TextMessage, chanList)

	if err != nil {
		log.Println("发送客户端列表给管理员失败:", err)
	}
}

func main() {
	http.HandleFunc("/ws", handleConnections)
	http.HandleFunc("/getQA", handleConnectionsQA)
	http.HandleFunc("/uploadxwefgdf", handleConnectionsUpload)

	log.Println("WebSocket 服务器启动在 :8989 端口")
	err := http.ListenAndServe(":8989", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
