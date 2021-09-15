# gitee.com/Ljolan/si-mqtt

#### 介绍
golang mqtt服务器，集群版【设计中】

#### 使用说明

- core 包为核心包
- 项目基础配置在core/config/config.toml里面
- 添加环境变量 SI_CFG_PATH = "配置文件路径" ，如果不配置，则默认使用core/config/config.toml配置
- 以package方式 运行 main.go即可

#### 设计可能选择的方案
1. mysql集群设计
2. zk+redis+自定义节点通讯
3. 静态配置启动【当前实现方案】

#### 待优化实现
1. 订阅客户端在busy的接收消息时，突然断开连接后，还在不停的发送，这个时候应该停止发送
2. 释放出栈消息的两个阶段可以添加批量删除，不然一个一个删除太慢了
3. session从数据库初始化的消息拉取，即客户端重连，未发布的消息重新拉取初始化内存队列
4. 断线后状态变更，是否需要丢弃内存中那份session，每次都从数据库中获取？
5. 断开连接后，需要处理完缓冲区内收到的消息
6. ...

#### 设计思路
![输入图片说明](https://images.gitee.com/uploads/images/2021/0903/231523_cbe216ec_3048600.png "客户端消息处理.excalidraw.png")
![输入图片说明](https://images.gitee.com/uploads/images/2021/0903/232740_351967e7_3048600.png "共享订阅集群通知.excalidraw.png")

#### 系统领域uml设计
[uml图、不同包中方法调用图](https://gitee.com/Ljolan/si-mqtt/tree/dev-cluster-v1/image)
