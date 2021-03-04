package shutdown_scripts

import "guest-test-infra/test_manager/test_manager"

func Setup()  {
	e2etestManager := test_manager.New()
	imageTest1 := e2etestManager.CreateImageTest()
	imageTest1.AddSkipImages("ubuntu")
	imageTest1.RunTests("TestXXX", "TestYYY")
	imageTest1.AddShutdownScript("path-to-script")

	imageTest2 := e2etestManager.CreateImageTest()
	imageTest2.RunTests("TestZZZ")

	e2etestManager.AddMetadata("key", "value")
	e2etestManager.AddImageTest(imageTest1, imageTest2)
}
