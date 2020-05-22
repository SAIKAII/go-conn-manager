package go_conn_manager

type server struct {
	multi multiplexing
}

func NewServer(m multiplexing) *server {
	return &server{multi: m}
}

func (s *server) Start(ipAddr string, port, headerLen, readMaxLen, writeMaxLen int)  {
	InitPackage(headerLen, readMaxLen, writeMaxLen)
	err := s.multi.init(ipAddr, port)
	if err != nil {
		panic(err)
	}

	go s.multi.waitEvent()
	s.multi.handleEvent()
}