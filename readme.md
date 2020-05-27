## TODO
- [x] 实现TCP数据封包与拆包
- [x] 实现Poll与Epoll两种多路复用
- [ ] 定时清理长时间无使用（无心跳包）的连接

## 笔记
1. 如果在调用EpollWait()时已有超过接收响应的切片大小，那么后续的EpollWait()调用将在剩余准备好的文件描述符集中进行循环。
2. POLLHUP与EPOLLHUP是同样标志，用于标记连接双方均已发送FIN包。
3. POLLRDHUP与EPOLLRDHUP是同样标志，用于标记连接另一方已经发送了FIN（即不再写）。
4. POLLERR与EPOLLERR是同样标志，用于标记接收到或已发送RST包。
5. 封包与拆包：
   1.  分头部与身体，头部记录数据长度，身体保存数据。头部占用2字节大小。
   2.  使用MSG_PEEK标记读取socket缓冲区中的数据，如果socket中数据长度不足头部中指定的长度，则终止此次读取操作，等待下一次更多数据到达，由于使用了MSG_PEEK标记，数据并不会被删除；如果socket中数据长度大于等于头部中指定的长度，则读取该长度的数据，读取完一个完整的包后如果socket中还有数据，则继续前面的操作，直到无法读取一个完整的包。
6. 由于Poll与Epoll不同，Poll多路复用需要在调用Poll方法前设置好需要监听的所有套接字，无法在监听过程中修改，所以每次Poll方法返回后，需要先把新增和要关闭的socket设置好，然后再进行下一次Poll监听。
### 产生RST包的情况：
1. 套接字缓冲区内还有数据未读取时关闭套接字会发送RST包
2. 彼方已关闭套接字，本地向套接字写数据会收到RST包
3. 彼方由于某些原因丢失了套接字信息，本地向套接字写数据会收到RST包
4. 与TCP三次握手有关，TCP之所以需要三次握手建立连接是基于以下这种情况：
   1. Client向Server发送SYN包（localIP1, localPort1, destIP1, destPort1）
   2. 这时网络中存在之前同样的（localIP1, localPort1, destIP1, destPort1）的SYN包（老），并且比这次的SYN包要先到Server，Server并不知道这是老的，Server转换自身状态为SYN-RECEIVED并发送SYN-ACK给Client
   3. Client收到该错误的SYN-ACK包后发送RST包给Server
   4. Server收到RST包后，把自己状态转换为LISTEN状态
   5. 后续正常的SYN包到达Server，后面就是正常的三次握手流程
5. 某一端系统（A）崩溃等原因造成丢失套接字信息：
   1. 重新启动后向另一方（B）发送SYN包建立连接
   2. B因为是ESTABLISHED状态，其认为该包是错误的，会向A返回正确ack的包
   3. A收到该包后检测B在之前已经打开了连接，所以A会发生RST包给B丢弃该连接
   4. 后续正常三次握手建立连接

## 参考
[POLLHUP vs POLLRDHUP](https://stackoverflow.com/questions/56177060/pollhup-vs-pollrdhup)