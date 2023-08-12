function __hry_preexec  --on-event fish_preexec
    hry -a -c "$argv"
end

function __hry_search
    if test -z "$argv"
        set h (hry)
        commandline -f repaint
        if test -n "$h"
            commandline -r $h
        end
    else
        set h (hry -s "$argv")
        commandline -f repaint
        if test -n "$h"
            commandline -r $h
        end
    end
end