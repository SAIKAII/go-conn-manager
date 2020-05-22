package go_conn_manager

type multiplexing interface {
	init(ipAddr string, port int) error
	waitEvent()
	handleEvent() error
}
