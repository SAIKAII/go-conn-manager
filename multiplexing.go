package go_conn_manager

type eventType int8

const (
	Event_Type_Connect eventType = iota
	Event_Type_Close
	Event_Type_In
	Event_Type_Error
)

type event struct {
	fd    int32
	event eventType
}

type multiplexing interface {
	SetHandler(h Handler)
	Init(ipAddr string, port int) error
	WaitEvent()
	HandleEvent() error
}
