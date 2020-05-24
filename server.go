package go_conn_manager

type server struct {
	multi multiplexing
}

func NewServer(m multiplexing) *server {
	return &server{multi: m}
}

func (s *server) Start(ipAddr string, port, headerLen, readMaxLen, writeMaxLen int, h Handler) {
	InitPackage(headerLen, readMaxLen, writeMaxLen)
	s.multi.SetHandler(h)
	err := s.multi.Init(ipAddr, port)
	if err != nil {
		panic(err)
	}

	go s.multi.WaitEvent()
	s.multi.HandleEvent()
}
