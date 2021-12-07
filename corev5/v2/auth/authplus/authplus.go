package authplus

type AuthPlus interface {
	//Verify  校验
	// param: authData 认证数据
	// return:
	//        d: 继续校验的认证数据
	//        continueAuth：true：成功，false：继续校验
	//        err != nil: 校验失败
	Verify(authData []byte) (d []byte, continueAuth bool, err error) // 客户端自己验证时，忽略continueAuth
}
