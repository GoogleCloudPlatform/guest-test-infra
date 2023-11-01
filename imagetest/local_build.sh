# usage: local_build.sh -o $outspath -c $cmdroot -s $suites_to_build
# the output path of the test files
outpath=.
# the root of the cmd folder, default assumes this script is run from imagetest
cmdroot=.
# the suites to build, space separated. all suites are built by default
suites=*

while getopts "o:c:s:" arg; do
  case $arg in 
    o) outpath=$OPTARG;;
    c) cmdroot=$OPTARG;;
    s) suites=$OPTARG;;
    *) echo "unknown arg"
  esac
done 
echo "outspath is $outpath"
echo "cmdroot is $cmdroot"
echo "suites being built are $suites"
go mod download
go build -o $outpath/wrapper.amd64 $cmdroot/cmd/wrapper/main.go
GOARCH=arm64 go build -o $outpath/wrapper.arm64 $cmdroot/cmd/wrapper/main.go
GOOS=windows GOARCH=amd64 go build -o $outpath/wrapp64.exe $cmdroot/cmd/wrapper/main.go
GOOS=windows GOARCH=386 go build -o $outpath/wrapp32.exe $cmdroot/cmd/wrapper/main.go
go build -o $outpath/manager $cmdroot/cmd/manager/main.go
cd test_suites
for suite in $suites; do
  [[ -d $suite ]] || continue
  cd $suite
  echo "building suite $suite"
  go test -c -tags cit || exit 1
  ./"${suite}.test" -test.list '.*' > $outpath/"${suite}_tests.txt" || exit 1
  mv "${suite}.test" $outpath/"${suite}.amd64.test" || exit 1
  GOARCH=arm64 go test -c -tags cit || exit 1
  mv "${suite}.test" "$outpath/${suite}.arm64.test" || exit 1
  GOOS=windows GOARCH=amd64 go test -c -tags cit || exit 1
  if [ -f "${suite}.test.exe" ]; then mv "${suite}.test.exe" "$outpath/${suite}64.exe" || exit 1; fi;
  GOOS=windows GOARCH=386 go test -c -tags cit || exit 1
  if [ -f "${suite}.test.exe" ]; then mv "${suite}.test.exe" "$outpath/${suite}32.exe" || exit 1; fi;
  cd ..
done
