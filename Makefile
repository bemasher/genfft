N = 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16

GEN_NOTW_FLAGS = -with-istride 1 -with-ostride 1 -standalone -dump-asched

.PHONY: all
all: $(N)

%::
	mkdir -p dft/dft$@
	-gen_notw.native -n $@ $(GEN_NOTW_FLAGS) dft/dft$@/dft.alst | egrep "^DV?K" >> dft/dft$@/dft.alst
	./genfft dft/dft$@/dft.alst > dft/dft$@/dft.go
	cp dft_test.go.src dft/dft$@/dft_test.go

%.go: %.alst

test:
	go test -v ./...

bench:
	go test -bench=.* -v ./...

clean:
	rm -rf dft/