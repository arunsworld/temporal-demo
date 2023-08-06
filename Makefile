IMAGE_NAME := arunsworld/temporal-demo
LD_FLAGS := -s -w
TARGETS := ./bankingdemo/customer:./bankingdemo/clearing-house:./bankingdemo/abbank:./bankingdemo/bcbank:./bankingdemo/money-laundering

pack-build:
	pack build ${IMAGE_NAME}:latest \
		--default-process customer \
		--env "BP_GO_TARGETS=${TARGETS}" \
		--env "BP_GO_BUILD_LDFLAGS=${LD_FLAGS}" \
		--builder paketobuildpacks/builder:base