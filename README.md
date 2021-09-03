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

#### 设计思路
![输入图片说明](https://images.gitee.com/uploads/images/2021/0903/231523_cbe216ec_3048600.png "客户端消息处理.excalidraw.png")
