.PHONY: all
all:
	@echo "Building client..."
	@make -f client.mk
	@echo "Building server..."
	@make -f server.mk
