import subprocess
result = subprocess.run(["go", "run", "./cmd/k3thrash", "report", "examples/trace.example.json"], text=True, capture_output=True, check=True)
print(result.stdout.strip())
