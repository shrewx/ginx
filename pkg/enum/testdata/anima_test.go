package testdata

import (
	"git.zdns.cn/ngo/servicex/pkg/enum"
	"github.com/go-courier/packagesx"
	"os"
	"testing"
)

func TestAnimal(t *testing.T) {
	pwd, _ := os.Getwd()
	pkg, err := packagesx.Load(pwd)
	if err != nil {
		t.Error(err)
		return
	}

	g := enum.NewEnumGenerator(pkg)
	g.Scan("Animal", "Job")
	g.Output(pwd)
}
