
.PHONY: all
all: repos/remote-repo repos/tagged-repo repos/commit-in-repo repos/tag-range-repo repos/annotated-tagged-repo

repos/remote-repo:
	./create-remote-repo.sh

repos/tagged-repo:
	./create-tagged-repo.sh

repos/commit-in-repo:
	./create-commit-in-repo.sh

repos/tag-range-repo:
	./create-tag-range-repo.sh

repos/annotated-tagged-repo:
	./create-annotated-tagged-repo.sh

clean:
	rm -rf repos/remote-repo repos/tagged-repo repos/commit-in-repo repos/tag-range-repo repos/annotated-tagged-repo
