function createColumn(parent, id) {
    let div = document.createElement('div');
    div.id = 'col-' + String(id);
    div.className = 'col';
    div.innerHTML = String(id)
    parent.appendChild(div)
    return div
}

function escapeHtml(unsafe) {
    return String(unsafe || "")
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

function getSyscallLink(name) {
    const { source_code, entry_point, man } = window.__syscalls__[name] || {}
    if (man) {
        return man
    }

    let link = source_code || ""
    if (entry_point) {
        link += "#:~:text=" + entry_point
    }
    return link
}

function renderArg(arg) {
    if (typeof arg != 'object') {
        arg = {Type: "string", Value: arg}
    }
    
    html = ""
    switch(arg.Type) {
        case "string":
            html = renderString(arg.Value, arg.Formated)
            break
        case "stat":
            html = renderStat(arg.Value, arg.Formated)
            break
        default:
            html = renderAnything(arg.Value, arg.Formated)
            break
    }

    cls = "strace_arg strace_arg_type_"+arg.Type
    return `<span class="${cls}">${html}</span>`
}

function renderAnything(smth, formated) {
    if (Array.isArray(smth)) {
        return escapeHtml(smth.map(String).join("|"))
    }
    if (typeof smth === 'object') {
        if (smth === null) {
            return "nil"
        }
        if ('Type' in smth && 'Value' in smth) {
            return renderArg(smth)
        }
        if ('Sec' in smth && 'Nsec' in smth) { // unix timeval or timespec
            return escapeHtml(new Date(1e3*smth.Sec + smth.Nsec/1e6).toJSON())
        }
        return renderStruct(smth, formated)
    }
    return escapeHtml(JSON.stringify(smth))
}

function renderString(str) {
    str = String(str)
    if (str.startsWith("\x7fELF")) {
        return escapeHtml("<bin content>")
    }
    if (str.length > 40) {
        str = str.substr(0, 40) + "..."
    }
    return escapeHtml(str)
}

function renderStruct(obj, formated, header) {
    header = header || "{...}"
    let popupHtml = renderStructPopup(obj, formated)
    return `<div class="strace_struct">
        <div class="strace_struct_header">${escapeHtml(header)}</div>
        ${popupHtml}
    </div>`
}

function renderStructPopup(obj, formated) {
    formated = formated || {}
    let html = ''
    for (let key in obj) {
        let val = formated[key] || obj[key]
        html += `<div class="strace_struct_row">${escapeHtml(key)}: ${renderAnything(val)}</div>`
    }
    return `<div class="strace_struct_content">${html}</div>`
}

function renderStat(obj, formated) {
    let mode = formated.Mode
    let size = formated.Size
    return renderStruct(obj, formated, `{mode=${mode}, size=${size}, ...}`)
}

function renderStraceItem(e) {
    const link = getSyscallLink(e.args.Syscall)
    let t = `<a class="strace_syscall_name" href="${escapeHtml(link)}" target="_blank">${escapeHtml(e.args.Syscall)}</a>`
    t += `<span class="strace_syscall_args">(${e.args.SyscallArgs.map(renderArg).join(', ')})</span>`
    if (e.args.Result) {
        t += `<span class="strace_result"> = ${renderArg(e.args.Result)}</span>`
    }
    if (e.args.Duration) {
        t += ` <span class="strace_result">&lt;${escapeHtml(String(e.args.Duration))}&gt;</span>`
    }
    return t
}

(function main(){
    const events = [] //window.__events__.traceEvents
    const main = document.querySelector("#main")
    const timeslotWidth = 10_000_000 // 10ms (10e6 ns)

    // let minTimeslot = Number.MAX_SAFE_INTEGER, maxTimeslot = 0;

    // It's needed to order processes by the time of first event
    // pid -> min(timestamp)
    const pidStarts = {}

    // timeslot -> pid -> events
    const data = {}

    for (const e of events) {
        pidStarts[e.pid] = Math.min(e.ts, pidStarts[e.pid] || Number.MAX_SAFE_INTEGER)

        const timeslot = Math.floor(e.ts / timeslotWidth)
        // minTimeslot = Math.min(minTimeslot, timeslot)
        // maxTimeslot = Math.max(maxTimeslot, timeslot)

        data[timeslot] = data[timeslot] || {}
        data[timeslot][e.pid] = data[timeslot][e.pid] || []
        data[timeslot][e.pid].push(e)
    }

    // pidOrder maintains an order of pid columns
    const pidOrder = [] //Object.entries(pidStarts).sort((a, b) => a[1] - b[1]).map(x => x[0])

    let html = '<div class="timeline">'

    html += '<div class="timeline_head">'
    for (let pid of pidOrder) {
        html += `<div class="timeline_head_cell">${pid}</div>`
    }
    html += '</div>'

    // for (let slot = minTimeslot; slot <= maxTimeslot; slot += 1) {
    //     html += '<div class="timeline_row">'
    //     for (let pid of pidOrder) {
    //         html += '<div class="timeline_cell">'
    //         if (data[slot] && data[slot][pid]) {
    //             for (const e of data[slot][pid]) {
    //                 let t = renderStraceItem(e)
    //                 timeFromStartMs = Math.round((e.ts - (minTimeslot * timeslotWidth)) / 1e6)
    //                 // strace_item_shadow is a clone that appears on hover
    //                 html += `<div class="strace_item" title="+${timeFromStartMs}ms">
    //                     <div class="strace_item_shadow">${t}</div>
    //                     ${t}
    //                 </div>`
    //             }
    //         }
    //         html += '</div>'
    //     }
    //     html += '</div>'
    // }
    html += '</div>'

    main.innerHTML += html

    const timelineNode = document.querySelector("#main .timeline")
    const timelineHeaderNode = timelineNode.querySelector(".timeline_head")

    let minTimeslot = 0
    let currentTimeslot = 0
    let currentEvents = {} // pid -> events
    const pids = {}

    function flush(nextTimeslot) {
        if (currentTimeslot == 0) {
            return
        }
        
        nextTimeslot = nextTimeslot || (currentTimeslot + 1)

        for (let slot = currentTimeslot; slot < nextTimeslot; slot += 1) {
            let html = ''
            for (let pid of pidOrder) {
                html += '<div class="timeline_cell">'
                if (slot == currentTimeslot && currentEvents[pid]) {
                    for (const e of currentEvents[pid]) {
                        let t = renderStraceItem(e)
                        timeFromStartMs = Math.round((e.ts - (minTimeslot * timeslotWidth)) / 1e6)
                        // strace_item_shadow is a clone that appears on hover
                        html += `<div class="strace_item" title="+${timeFromStartMs}ms">
                            <div class="strace_item_shadow">${t}</div>
                            ${t}
                        </div>`
                    }
                }
                html += '</div>'
            }
            appendChild(timelineNode, 'timeline_row', html)
        }
        currentEvents = []
    }

    function addEvent(e) {
        const timeslot = Math.floor(e.ts / timeslotWidth)
        if (timeslot > currentTimeslot) {
            flush(timeslot)
        }
        if (0 == minTimeslot) {
            minTimeslot = timeslot
        }
        currentTimeslot = timeslot
        currentEvents[e.pid] = currentEvents[e.pid] || []
        currentEvents[e.pid].push(e)

        // if the PID is unknown add a column for it
        if (!pids[e.pid]) {
            pids[e.pid] = true
            pidOrder.push(e.pid)
            appendChild(timelineHeaderNode, 'timeline_head_cell', e.pid)
        }
    }

    const eventSource = new EventSource("/events")

    eventSource.addEventListener('message', function (event) {
        const e = JSON.parse(event.data)
        addEvent(e)
    })
    eventSource.addEventListener('fin', function () {
        eventSource.close()
        flush()
    })
    eventSource.onerror = function (err) {
        console.error("EventSource failed:", err)
        eventSource.close()
        flush()
    }
})()

function appendChild(parent, className, html){
    const el = document.createElement('div')
    el.classList.add(className)
    el.innerHTML = String(html)
    if (parent) {
        parent.append(el)
    }
    return el
}

/**
 * Format bytes as human-readable text.
 * 
 * @param bytes Number of bytes.
 * @param si True to use metric (SI) units, aka powers of 1000. False to use 
 *           binary (IEC), aka powers of 1024.
 * @param dp Number of decimal places to display.
 * 
 * @return Formatted string.
 * 
 * Source: https://stackoverflow.com/a/14919494/502860
 */
function humanFileSize(bytes, si=false, dp=1) {
    const thresh = si ? 1000 : 1024;
  
    if (Math.abs(bytes) < thresh) {
      return bytes + ' B';
    }
  
    const units = si 
      ? ['kB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'] 
      : ['KiB', 'MiB', 'GiB', 'TiB', 'PiB', 'EiB', 'ZiB', 'YiB'];
    let u = -1;
    const r = 10**dp;
  
    do {
      bytes /= thresh;
      ++u;
    } while (Math.round(Math.abs(bytes) * r) / r >= thresh && u < units.length - 1);
  
  
    return bytes.toFixed(dp) + ' ' + units[u];
}
