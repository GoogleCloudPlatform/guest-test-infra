package image_validation

import "guest-test-infra/test_manager/test_manager"

func Setup() {
	e2etestMananger := test_manager.New()
	imageTest1 := e2etestMananger.CreateImageTest()
	imageTest2 := e2etestMananger.CreateImageTest()
	imageTest1.AddSkipImages()
	imageTest1.RunTests("TestXXX", "TestYYY")
	imageTest2.RunTests("TestZZZ")
	e2etestMananger.AddImageTest(imageTest1, imageTest2)
}
