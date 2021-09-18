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
~~1. 发送给客户端的pkid应该专属，不能用上发来的那个旧pkid~~
2. 
3. 释放出栈消息的两个阶段可以添加批量删除，不然一个一个删除太慢了
~~4. session从数据库初始化的消息拉取~~
5. 断线后状态变更，是否需要丢弃内存中那份，每次都从数据库中获取？ 
   > --- 当前节点先不删，等到了过期时间再系统自动删除，当节点在其它节点连接时，
        其它节点会通知这边删除旧session，其他节点那边从数据库获取再初始化，
        如果又在自己这个节点上连接，则会继续使用这个session。需要注意qos=0的消息
   
~~6.断开连接后，需要处理完输入缓冲区内收到的消息~~
7. 消息过期间隔
~~8. session过期间隔，disconnect中可以重新设置过期时间~~
9.主题别名
10.Request/Response 模式
11. 订阅标识符
~~12.订阅选项 NoLocal、Retain As Publish、Retain Handling处理~~
13.主题别名(Topic Alias)处理
14.流控
15.Receive Maximum 属性
16.补充Reason string
17.Maximum Packet Size处理
18.遗嘱延迟发生处理
19. ...

#### 设计思路
![输入图片说明](https://images.gitee.com/uploads/images/2021/0903/231523_cbe216ec_3048600.png "客户端消息处理.excalidraw.png")
![输入图片说明](https://images.gitee.com/uploads/images/2021/0903/232740_351967e7_3048600.png "共享订阅集群通知.excalidraw.png")

#### 系统领域uml设计
[uml图、不同包中方法调用图](https://gitee.com/Ljolan/si-mqtt/tree/dev-cluster-v1/image)
