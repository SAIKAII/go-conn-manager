## TODO
- [x] 实现Poll与Epoll两种多路复用
- [ ] 定时清理长时间无使用（无心跳包）的连接

## 笔记
1. 如果在调用EpollWait()时已有超过接收响应的切片大小，那么后续的EpollWait()调用将在剩余准备好的文件描述符集中进行循环。
2. POLLHUP与EPOLLHUP是同样标志，用于标记连接双方均已发送FIN包。
3. POLLRDHUP与EPOLLRDHUP是同样标志，用于标记连接另一方已经发送了FIN（即不再写）。
4. POLLERR与EPOLLERR是同样标志，用于标记接收到或已发送RST包。

## 参考
[POLLHUP vs POLLRDHUP](https://stackoverflow.com/questions/56177060/pollhup-vs-pollrdhup)