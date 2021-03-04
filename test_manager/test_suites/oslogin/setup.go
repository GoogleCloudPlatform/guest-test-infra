package oslogin

import "guest-test-infra/test_manager/test_manager"

func Setup()  {
	e2etestMananger := test_manager.New()
	imageTest1 := e2etestMananger.CreateImageTest()
	imageTest1.AddShutdownScript("script")
	imageTest1.RunAllTests()
	e2etestMananger.AddImageTest(imageTest1)
}
