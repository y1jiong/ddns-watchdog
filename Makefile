.PHONY: all
all:
	@echo "Building client..."
	@make -f Makefile.client
	@echo "Building server..."
	@make -f Makefile.server
