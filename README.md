# gitee.com/Ljolan/si-mqtt

#### 介绍
golang mqtt服务器，集群版，目前支持DB集群和直连集群

#### 使用说明

- corev5 包为核心包
- 项目基础配置在config/config.toml里面
- 添加环境变量 SI_CFG_PATH = "配置文件路径" ，如果不配置，则默认使用config/config.toml配置
- 以package方式 运行 main.go即可

##### 客户端调用测试
- 可以使用开源库paho-golang的v5编解码client进行测试
- [paho库的客户端测试][https://github.com/eclipse/paho.golang/tree/master/paho]

```
// 本地启动broker服务之后
// 可以将此代码直接粘贴在paho的client_test.go里面运行即可测试

func TestClientTest(t *testing.T) {
 	conn, _ := net.Dial("tcp","127.0.0.1:1883")
 	c := NewClient(ClientConfig{
 		Conn:                       conn,
 		Router: NewSingleHandlerRouter(func(p *Publish) {
 			fmt.Println(p.String())
 		}),
 	})
 	c.serverProps.SharedSubAvailable = true
 	require.NotNil(t, c)
 	c.SetDebugLogger(log.New(os.Stderr, "CONNECT: ", log.LstdFlags))
 
 	cp := &Connect{
 		KeepAlive:  30,
 		ClientID:   "testClient",
 		CleanStart: true,
 		Properties: &ConnectProperties{
 			ReceiveMaximum: Uint16(200),
 		},
 		WillMessage: &WillMessage{
 			Topic:   "will/topic",
 			Payload: []byte("am gone"),
 		},
 		WillProperties: &WillProperties{
 			WillDelayInterval: Uint32(200),
 		},
 	}
 
 	ca, err := c.Connect(context.Background(), cp)
 	require.Nil(t, err)
 	assert.Equal(t, uint8(0), ca.ReasonCode)
 
 	time.Sleep(10 * time.Millisecond)
 
 	s := &Subscribe{
 		Subscriptions: map[string]SubscribeOptions{
 			"test/1": {QoS: 1},
 			"test/2": {QoS: 2},
 			"test/3": {QoS: 0},
 			"$share/aa/test/1": {QoS: 0},
 		},
 	}
 
 	_, err = c.Subscribe(context.Background(), s)
 	require.Nil(t, err)
 	//assert.Equal(t, []byte{1, 2, 0}, sa.Reasons)
 
 	time.Sleep(10 * time.Millisecond)
 	var p *Publish
 	p = &Publish{
 		Topic:   "test/0",
 		QoS:     0,
 		Payload: []byte("test payload"),
 	}
 	
 	_, err = c.Publish(context.Background(), p)
 	require.Nil(t, err)
 	
 	time.Sleep(10 * time.Millisecond)
 
 	p = &Publish{
 		Topic:   "test/1",
 		QoS:     1,
 		Payload: []byte("test payload"),
 	}
 
 	pa, err := c.Publish(context.Background(), p)
 	require.Nil(t, err)
 	assert.Equal(t, uint8(0), pa.ReasonCode)
 
 	time.Sleep(20 * time.Millisecond)
 	p = &Publish{
 		Topic:   "test/2",
 		QoS:     2,
 		Payload: []byte("test payload"),
 	}
 	
 	pr, err := c.Publish(context.Background(), p)
 	require.Nil(t, err)
 	assert.Equal(t, uint8(0), pr.ReasonCode)
 	
 	time.Sleep(30 * time.Millisecond)
 }
```

#### 目前支持的方案
##### 1. Mongo集群设计
##### 2. Mysql集群设计
- 新增集群共享订阅数据自动合并方法，简易代码 [cluster/stat/colong/auto_compress_sub/factory.go](https://gitee.com/Ljolan/si-mqtt/blob/dev-cluster-v1/cluster/stat/colong/auto_compress_sub/factory.go)

  > 采用DB存储集群消息的方式，可通过多主的方式提供写服务，拉取数据走从即可，可以防止主节点单点故障，并且不会全部broker连在一个DB实例上。[具体知识点可查看DDIA](https://ddia.vonng.com/#/part-ii)
  > DB集群节点数据复制【反熵过程】延迟高峰期会比较大，对数据实时比较敏感的，建议在物模型的数据添加一个时间戳，订阅者收到数据根据时间戳决定是否丢弃
##### 3. 静态配置启动

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

~~16. 遗嘱延迟发送处理~~
    
17. 订阅了共享订阅的客户端掉线后，导致其它节点依旧会发送共享订阅给这个客户端，需要处理，等重连后重新订阅此共享订阅

18. 共享订阅保留问题【mysql方式已解决】
    
19. .....

#### 旧设计思路
![输入图片说明](https://images.gitee.com/uploads/images/2021/0903/231523_cbe216ec_3048600.png "客户端消息处理.excalidraw.png")
![输入图片说明](https://images.gitee.com/uploads/images/2021/0903/232740_351967e7_3048600.png "共享订阅集群通知.excalidraw.png")

#### 系统领域uml设计
[uml图、不同包中方法调用图](https://gitee.com/Ljolan/si-mqtt/tree/dev-cluster-v1/image)


[https://github.com/eclipse/paho.golang/tree/master/paho]: https://github.com/eclipse/paho.golang/tree/master/paho