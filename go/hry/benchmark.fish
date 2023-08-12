set cmd1 'go install github.com/abcdlsj/share/go/hry@latest'
set cmd2 'cat $HOME/.config/fish/history.hry > /dev/null'

./hry -clear
set st (date +%s)
for i in (seq 1 1000)
    ./hry -a -c "$cmd1/$i"
    ./hry -a -c "$cmd2/$i"
end
set et (date +%s)


echo "append 2000 cmd"
echo "   cost: " (math $et - $st) "s"
echo "   avg: " (math (math $et - $st) x 1000 / 2000) "ms"

echo -e "\nmatch 2000 cmd"
set st (date +%s)
for i in (seq 1 1000)
    ./hry -s "go install" > /dev/null
    ./hry -s "cat $HOME/.config/fish/history.hry" > /dev/null
end
set et (date +%s)

echo "   cost: " (math $et - $st) "s"
echo "   avg: " (math (math $et - $st) x 1000 / 2000) "ms"