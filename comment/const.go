package comment

var (
	ServerVersion         = "3.1.0"
	DefaultKeepAlive      = 300
	DefaultConnectTimeout = 2
	DefaultAckTimeout     = 20
	DefaultTimeoutRetries = 3
	//下面两个最好一样,要是修改了要去这两个文件中（sessions/memprovider.go和topics/memtopics.go）修改对应的
	//因为不能在那里面要用这个，不然会引起相互依赖的
	//还有redis的，也在上面两个文件包下
	DefaultSessionsProvider = "mem" //保存至内存中
	DefaultTopicsProvider   = "mem"

	//下面两个修改了，也要要去auth/mock.go文件中修改
	DefaultAuthenticator = "default" //关闭身份验证
	DefaultAuthOpen      = false
)
