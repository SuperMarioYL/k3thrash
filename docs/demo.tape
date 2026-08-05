Output assets/demo/k3thrash-demo.gif
Set FontSize 15
Set Width 1200
Set Height 620
Set Padding 20
Set Theme Dracula

# 10-minute happy path: a CN K3 lab operator runs k3thrash on a saved trace
# from a drifting-tps session to get the shareable verdict + sparkline.

Type "k3thrash --version"
Enter
Sleep 1500ms

Type "k3thrash report examples/trace.example.json"
Enter
Sleep 500ms

# report renders verdict one-liner + detail + sparkline
Sleep 3500ms

Type "# verdict: re-read rate 3.0x reuse -> pathological thrash"
Enter
Sleep 1500ms

Sleep 1s
