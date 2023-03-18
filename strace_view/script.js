function createColumn(parent, id) {
    let div = document.createElement('div');
    div.id = 'col-' + String(id);
    div.className = 'col';
    div.innerHTML = String(id)
    parent.appendChild(div)
    return div
}

function escapeHtml(unsafe) {
    return (unsafe || "")
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

function renderStraceItem(e) {
    const link = getSyscallLink(e.args.Syscall)
    let t = `<a class="strace_syscall_name" href="${escapeHtml(link)}" target="_blank">${escapeHtml(e.args.Syscall)}</a>`
    t += `<span class="strace_syscall_args">(${escapeHtml(e.args.SyscallArgs)})</span>`
    if (e.args.Result) {
        t += `<span class="strace_result"> = ${escapeHtml(e.args.Result)}</span>`
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
            pidOrder.push(e.pid)
            appendChild(timelineHeaderNode, 'timeline_head_cell', e.pid)
        }
    }

    const eventSource = new EventSource("/events")

    eventSource.addEventListener('message', function (event) {
        const e = JSON.parse(event.data)
        addEvent(e)
        // main.innerHTML += `<div>${event.data}</div>`
        // const newElement = document.createElement("li")
        // const eventList = document.getElementById("list")
        // newElement.textContent = `message: ${event.data}`
        // eventList.appendChild(newElement)
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
        parent.appendChild(el)
    }
    return el
}
