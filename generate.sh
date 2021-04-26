#!/bin/bash

for N in {2..16}
do
    echo "generating dft N=${N}"
    gen_notw.native \
        -n ${N} -standalone -with-istride 1 -with-ostride 1 \
        -dump-asched dft/float_${N}.alst > dft/float_${N}.cout
    
    gen_notw_c.native \
        -n ${N} -standalone -with-istride 1 -with-ostride 1 \
        -dump-asched dft/cmplx_${N}.alst > dft/cmplx_${N}.cout
done