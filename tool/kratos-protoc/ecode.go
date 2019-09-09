package main

import (
	"os/exec"

	"github.com/urfave/cli"
)

const (
	_getEcodeGen = "go get -u github.com/yuanfeng0905/oasis-kratos/tool/protobuf/protoc-gen-ecode"
	_ecodeProtoc = "protoc --proto_path=%s --proto_path=%s --proto_path=%s --ecode_out=:."
)

func installEcodeGen() error {
	if _, err := exec.LookPath("protoc-gen-ecode"); err != nil {
		if err := goget(_getEcodeGen); err != nil {
			return err
		}
	}
	return nil
}

func genEcode(ctx *cli.Context) error {
	return generate(ctx, _ecodeProtoc)
}
