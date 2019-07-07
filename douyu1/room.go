package douyu

import (
	"encoding/binary"
	"net"
	"time"

	"github.com/xtlx2000/golib/log"

	"github.com/xtlx2000/douyuLib/douyu/protocol"
)

const (
	addr    = "openbarrage.douyutv.com:8601" // 服务器地址
	heartbe = "30s"                          // 心跳时间
)

// room对象可以与服务器建立tcp连接，并与之通信
type Room struct {
	RoomId                                  string
	conn                                    net.Conn
	login                                   chan bool
	barrageSwitch, allMsgSwitch, joinSwitch bool
	barrage, allMsg, join                   chan map[string]string
	bool
	logout chan bool
}

// 返回一个room指针
func NewRoom(roomId string) *Room {
	return &Room{RoomId: roomId, login: make(chan bool), logout: make(chan bool)}
}

// 运行这个room
func (r *Room) Run() error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	r.conn = conn

	conn.Write(protocol.MsgToByte(map[string]string{
		"type":   "loginreq",
		"roomid": r.RoomId,
	}))
	go receiveMsg(r)
	go r.keepConnection()
	<-r.logout
	return nil
}

// 接收服务器返回的消息
func receiveMsg(r *Room) {
	for {
		// 读取协议头
		h := make([]byte, protocol.HeadLen*2+protocol.MsgTypeLen+protocol.KeepLen)
		n, err := r.conn.Read(h)
		if err != nil {
			log.Errorf("[ERROR]:%v", err)
			return
		}
		// log.Println("data", h[:n])
		// 读取body
		b := make([]byte, int(binary.LittleEndian.Uint32(h[0:4]))-int(protocol.HeadLen+protocol.MsgTypeLen+protocol.KeepLen))
		n, err = r.conn.Read(b)
		if err != nil {
			log.Errorf("[ERROR]:%v", err)
			return
		}

		// log.Println("data", len(b[:n]), b[:n])
		// return
		data, err := protocol.ByteToMsg(b[:n])
		if err != nil {
			log.Errorf("[ERROR]:%v", err)
			return
		}
		switch data["type"] {
		case "loginres":
			r.login <- true
			// 加入组消息
			r.conn.Write(protocol.MsgToByte(map[string]string{
				"type": "joingroup",
				"rid":  r.RoomId,
				"gid":  "-9999",
			}))
			log.Warningf("机器人登录直播间完成")
		case "chatmsg":
			// 弹幕消息
			if r.barrageSwitch {
				r.barrage <- data
			}
		case "uenter":
			// 有人进入房间
			if r.joinSwitch {
				r.join <- data
			}

		default:
			// log.Println("Unkown type data: %v", data)
			continue
		}
		if r.allMsgSwitch {
			r.allMsg <- data
		}
	}
}

// 用户进入直播间
func (r *Room) JoinRoom(chanSize int) <-chan map[string]string {
	r.joinSwitch = true
	r.join = make(chan map[string]string, chanSize)
	return r.join
}

// 接收弹幕消息
func (r *Room) ReceiveBarrage(chanSize int) <-chan map[string]string {
	r.barrageSwitch = true
	r.barrage = make(chan map[string]string, chanSize)
	return r.barrage
}

// 接收所有消息
func (r *Room) ReceiveAll(chanSize int) <-chan map[string]string {
	r.allMsgSwitch = true
	r.allMsg = make(chan map[string]string, chanSize)
	return r.allMsg
}

// 客户端与服务器保持连接
func (r *Room) keepConnection() {
	<-r.login
	close(r.login)
	for {
		r.conn.Write(protocol.MsgToByte(map[string]string{
			"type": "mrkl",
		}))
		t, _ := time.ParseDuration(heartbe)
		time.Sleep(t)
	}
}
