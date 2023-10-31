# rootpath should be an absolute path where you want test output files storedm, such as /home/username
# suite is the name of the test suite
rootpath=$1
suite=$2
go build -o $rootpath/wrapper.amd64 ./cmd/wrapper/main.go
go build -o $rootpath/manager ./cmd/manager/main.go
cd test_suites
cd $suite
go test -c -tags cit
./"${suite}.test" -test.list '.*' > $rootpath/"${suite}_tests.txt"
mv "${suite}.test" $rootpath/"${suite}.amd64.test"
