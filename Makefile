.PHONY: hooks

hooks:
	git config core.hooksPath .githooks
	git config commit.template .gitmessage

