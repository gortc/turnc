module github.com/gortc/turnc/e2e/turn-client

go 1.12

require (
	go.uber.org/zap v1.10.0
	gortc.io/turn v0.10.0
	gortc.io/turnc v0.0.6
)

replace gortc.io/turnc => ../../
