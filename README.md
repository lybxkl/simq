# gitee.com/Ljolan/si-mqtt

#### 介绍
golang mqtt服务器，集群版，目前支持DB集群和直连集群

#### 使用说明

- corev5 包为核心包
- 项目基础配置在config/config.toml里面
- 添加环境变量 SI_CFG_PATH = "配置文件路径" ，如果不配置，则默认使用config/config.toml配置
- 以package方式 运行 main.go即可

#### 目前支持的方案
##### 1. Mongo集群设计
##### 2. 静态配置启动

#### 多节点启动
![输入图片说明](https://images.gitee.com/uploads/images/2021/0928/234746_6f1bc35d_3048600.png "QQ图片20210928234729.png")
##### MQTTX使用
![输入图片说明](https://images.gitee.com/uploads/images/2021/0928/234807_0e98852f_3048600.png "QQ图片20210928234725.png")

#### 待优化实现
1. ~~发送给客户端的pkid应该专属，不能用上发来的那个旧pkid~~
2. 释放出栈消息的两个阶段可以添加批量删除，不然一个一个删除太慢了
3. ~~session从数据库初始化的消息拉取~~
4. 断线后状态变更，是否需要丢弃内存中那份，每次都从数据库中获取？ 
   
   > --- 当前节点先不删，等到了过期时间再系统自动删除，当节点在其它节点连接时，
        其它节点会通知这边删除旧session，其他节点那边从数据库获取再初始化，
        如果又在自己这个节点上连接，则会继续使用这个session。需要注意qos=0的消息
   
5. ~~断开连接后，需要处理完输入缓冲区内收到的消息~~

6. 消息过期间隔
   
7. ~~session过期间隔，disconnect中可以重新设置过期时间~~

8. Request/Response 模式 ---（客户端处理）

9. ~~订阅标识符~~

10. ~~订阅选项 NoLocal、Retain As Publish、Retain Handling处理~~
    
11. ~~主题别名(Topic Alias)处理~~

12. ~~流控~~

13. Receive Maximum 属性

14. 补充Reason string

15. Maximum Packet Size处理

16. 遗嘱延迟发送处理
    
17. 订阅了共享订阅的客户端掉线后，导致其它节点依旧会发送共享订阅给这个客户端，需要处理，等重连后重新订阅此共享订阅

18. .....

#### 旧设计思路
![输入图片说明](https://images.gitee.com/uploads/images/2021/0903/231523_cbe216ec_3048600.png "客户端消息处理.excalidraw.png")
![输入图片说明](https://images.gitee.com/uploads/images/2021/0903/232740_351967e7_3048600.png "共享订阅集群通知.excalidraw.png")

#### 系统领域uml设计
[uml图、不同包中方法调用图](https://gitee.com/Ljolan/si-mqtt/tree/dev-cluster-v1/image)
