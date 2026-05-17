
mkdir -p "$TMPDIR/wp_results" && benches=(
     BenchmarkNoPool BenchmarkErrGroup BenchmarkAntsPool BenchmarkSemaphorePool
     BenchmarkPreallocPool BenchmarkStaticPool BenchmarkRoundRobinPool
     BenchmarkCapRR_NumCPU_half BenchmarkCapRR_NumCPU_x1 BenchmarkCapRR_NumCPU_x2
     BenchmarkCapRR_NumCPU_x4 BenchmarkCapRR_NumCPU_x8
     BenchmarkCapStatic_NumCPU_half BenchmarkCapStatic_NumCPU_x1 BenchmarkCapStatic_NumCPU_x2
     BenchmarkCapStatic_NumCPU_x4 BenchmarkCapStatic_NumCPU_x8
     BenchmarkVariableNoPool BenchmarkVariableErrGroup BenchmarkVariableAntsPool
     BenchmarkVariablePreallocPool BenchmarkVariableStaticPool BenchmarkVariableRoundRobinPool
     BenchmarkPinnedStaticPool BenchmarkPinnedRoundRobinPool
   ) && for b in "${benches[@]}"; do
     echo "[$(date +%T)] cooldown 60s before $b"
     sleep 60
     echo "[$(date +%T)] running $b"
     go test -run=^$ -bench="^${b}$" -benchmem -benchtime=5s -count=6 -timeout=10m >
   "$TMPDIR/wp_results/${b}.txt" 2>&1
   done && echo "[$(date +%T)] all done"
