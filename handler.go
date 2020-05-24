package go_conn_manager

type Handler interface {
	OnConnect(*Conn)         // 创建连接时调用
	OnMessage(*Conn, []byte) // 套接字有消息可读时调用
	OnClose(*Conn) error     // 主动关闭连接或超时无心跳包时调用
	OnError(*Conn)           // 套接字发生了错误，一般是接收到RST
}
