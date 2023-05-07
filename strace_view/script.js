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
    
    let html = "", child;
    switch(arg.Type) {
        case "string":
            child = renderString(arg.Value, arg.Formated)
            break
        case "stat":
            child = renderStat(arg.Value, arg.Formated)
            break
        default:
            html = renderAnything(arg.Value, arg.Formated)
            break
    }

    const elem = el('strace_arg', html)
    elem.classList.add('strace_arg_type_' + arg.Type)
    if (child) {
        elem.append(child)
    }
    return elem
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
        str = "<bin content>"
    }
    if (str.length > 40) {
        str = str.substr(0, 40) + "..."
    }
    const span = document.createElement('span')
    span.textContent = str
    return span
}


function renderStruct(obj, formated, header) {
    header = header || "{...}"
    let popupHtml = renderStructPopup(obj, formated)
    const container = el('strace_struct')
    const head = el('strace_struct_header')
    head.textContent = header
    container.append(head)
    tippy(container, {
        // trigger: 'click',
        content: popupHtml,
        // appendTo: container,
        appendTo: () => document.body,
        allowHTML: true,
        interactive: true,
        placement: 'bottom-start',
        offset: [0, 0],
        arrow: false,
    })
    return container
    // return `<div class="strace_struct">
    //     <div class="strace_struct_header">${escapeHtml(header)}</div>
    //     ${popupHtml}
    // </div>`
}

function renderStructPopup(obj, formated) {
    formated = formated || {}
    let html = ''
    for (let key in obj) {
        let val = formated[key] || obj[key]
        html += `<div class="st111race_struct_row">${escapeHtml(key)}: ${renderAnything(val)}</div>`
    }
    return `<div class="str111ace_struct_content">${html}</div>`
}

function renderStat(obj, formated) {
    let mode = formated.Mode
    let size = formated.Size
    return renderStruct(obj, formated, `{mode=${mode}, size=${size}, ...}`)
}

function renderStraceItem(e) {
    const item = el('strace_item')

    const a = document.createElement('a')
    a.classList.add('strace_syscall_name')
    a.href = getSyscallLink(e.args.Syscall)
    a.target = '_blank'
    a.textContent = e.args.Syscall
    item.append(a)
    
    const args = el('strace_syscall_args')
    for (let x of e.args.SyscallArgs) {
        let arg = renderArg(x)
        args.append(arg)
    }
    item.append(args)

    if (e.args.Result) {
        const res = el('strace_result')
        res.append(renderArg(e.args.Result))
        item.append(res)
    }
    return item
}

(function memstat(){
    if (!performance || !performance.memory) {
        return
    }

    const el = document.querySelector("#memstat")
    setInterval(() => {
        el.textContent = humanFileSize(performance.memory.totalJSHeapSize)
        let usagePerc = Math.round(100 * performance.memory.totalJSHeapSize / performance.memory.jsHeapSizeLimit)
        if (usagePerc > 0) {
            el.textContent += " (" + usagePerc + "%)"
        }
    }, 1000)
})();

const UI = {
    rowHeight: 24,
    cellHeightMin: 5,
    borderHeight: 1,
};

(function main(){
    const events = []
    const main = document.querySelector("#main")
    const timeslotWidth = 10_000_000 // 10ms (10e6 ns)

    // let minTimeslot = Number.MAX_SAFE_INTEGER, maxTimeslot = 0;

    // timeslot -> pid -> events
    const data = {}

    data.addEvent = function(e) {
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

    // heights of timeslot blocks in pixels
    const layout = []

    function flush(nextTimeslot) {
        if (currentTimeslot == 0) {
            return
        }
        
        nextTimeslot = nextTimeslot || (currentTimeslot + 1)

        // calc timeslots hight
        const biggestCell = Math.max(...pidOrder.map(pid => (currentEvents[pid] || []).length));
        layout.push(
            (biggestCell > 0 ? UI.rowHeight*biggestCell : UI.cellHeightMin) + UI.borderHeight
        )
        for (let slot = currentTimeslot+1; slot < nextTimeslot; slot += 1) {
            layout.push(UI.cellHeightMin + UI.borderHeight)
        }

        for (let slot = currentTimeslot; slot < nextTimeslot; slot += 1) {
            let row = el('timeline_row')
            row.setAttribute('timeslot', slot)
            row.setAttribute('i', slot-minTimeslot)
            
            const height = layout[slot-minTimeslot]
            row.style.height = height+'px'

            for (let pid of pidOrder) {
                let cell = el('timeline_cell')
                row.append(cell)

                if (slot == currentTimeslot && currentEvents[pid]) {
                    for (const e of currentEvents[pid]) {
                        // timeFromStartMs = Math.round((e.ts - (minTimeslot * timeslotWidth)) / 1e6)
                        let item = renderStraceItem(e)
                        // cell.append(item)
                    }
                }
            }

            timelineNode.append(row)
            // appendChild(timelineNode, 'timeline_row', html)
        }
        
        currentEvents = {}
    }

    function addEvent(e) {
        data.addEvent(e)

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


    ;(function scroll() {
        function calcScrollHeight() {
            return layout.reduce((a, b) => a + b, 0)
        }
    
        window.addEventListener('scroll', () => {
            // if (window.innerHeight + window.scrollY >= document.body.offsetHeight - 1000 && ready)
            console.log('scroll = ' + window.scrollY + '; scrollHeight = ' + calcScrollHeight())
            window.layout = layout
        })
    }())
})();

function el(className, html) {
    const el = document.createElement('div')
    el.classList.add(className)
    if (html) {
        el.innerHTML = String(html)
    }
    return el
}

function appendChild(parent, className, html){
    const el = document.createElement('div')
    el.classList.add(className)
    el.innerHTML = String(html)
    if (parent) {
        parent.append(el)
    }
    return el
}

function prependChild(parent, className, html){
    const el = document.createElement('div')
    el.classList.add(className)
    el.innerHTML = String(html)
    if (parent) {
        parent.prepend(el)
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
