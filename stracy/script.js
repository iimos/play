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
        case "flags":
            child = renderFlags(arg.Value, arg.Formated)
            break
        case "stack_t":
            child = renderStruct(arg.Value, arg.Formated)
            break
        case "msghdr":
            child = renderStruct(arg.Value, arg.Formated)
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

function renderFlags(arr, formated) {
    if (!arr || !arr.length) {
        return "0"
    }
    // strip common prefix
    let parts = arr[0].split('_')
    if (parts.length > 1) {
        let prefix = parts[0] + '_'
        arr = arr.map(x => x.replace(prefix, ''))
    }
    return escapeHtml(arr.join("|"))
}

function renderString(str) {
    str = String(str)
    if (str.startsWith("\x7fELF")) {
        str = "<binary>"
    }
    if (str.length > 40) {
        str = str.substr(0, 40) + "..."
    }
    const span = document.createElement('span')
    span.textContent = str
    return span
}

function renderStruct(obj, formated, header) {
    let empty = false
    if (!header) {
        if (obj === null || obj === undefined) {
            header = "null"
            empty = true
        } else if (Object.keys(obj).length === 0) {
            header = "{}"
            empty = true
        } else {
            header = "{...}"
        }
    }

    const popupHtml = renderStructPopup(obj, formated)
    const container = el('strace_struct')
    const head = el('strace_struct_header')
    head.textContent = header
    container.append(head)

    if (!empty) {
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
    }
    return container
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
    a.title = JSON.stringify(e)
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

// draft
class Timeline {
    #rootNode;
    #headerNode;
    #slotNodes = {}

    data = {}; // timeslot -> pid -> events

    #timeslotDuration = 10_000_000; // 10ms (10e6 ns)
    #PIDOrder = [];
    #PIDIndexes = {};
    #minTimeslot = 0;
    #currentTimeslot = 0;

    // heights of timeslot blocks in pixels
    #layout = []

    constructor(rootNode) {
        const that = this
        this.observer = new IntersectionObserver((entries, observer) => {
            entries.forEach(entry => {
                const row = entry.target
                const timeslot = row.getAttribute("data-timeslot")
                if (entry.isIntersecting) {
                    that.#renderContent(timeslot)
                } else {
                    // that.#hideContent(timeslot)
                }
            })
        }, {
            root: null,
            rootMargin: '0px',
            threshold: 0,
        })

        this.#rootNode = rootNode
        this.#headerNode = el('timeline_head')
        this.#rootNode.append(this.#headerNode)
        // window.addEventListener('scroll', debounce(this.#render.bind(this), 200))
    }

    appendEvent(e) {
        const timeslot = Math.floor(e.ts / this.#timeslotDuration)

        // store event into this.data
        this.data[timeslot] = this.data[timeslot] || {}
        this.data[timeslot][e.pid] = this.data[timeslot][e.pid] || []
        this.data[timeslot][e.pid].push(e)

        if (!this.#minTimeslot) {
            // here we assume data is appended chronologically
            this.#minTimeslot = timeslot
        }
        if (!this.#currentTimeslot) {
            this.#currentTimeslot = timeslot
        }

        if (timeslot < this.#currentTimeslot) {
            console.error("got event with slot < currentTimeslot; timeslot="+timeslot+"; currentTimeslot="+this.#currentTimeslot)
            return
        }

        this.addPID(e.pid)
        for (let slot = this.#currentTimeslot; slot <= timeslot; slot += 1) {
            this.#adjustPlaceholder(slot)
        }
        this.#currentTimeslot = timeslot
    }

    finish() {
        this.#currentTimeslot = this.#currentTimeslot + 1
        for (let slot = this.#currentTimeslot; slot <= this.#currentTimeslot; slot += 1) {
            this.#adjustPlaceholder(slot)
        }
    }

    addPID(pid) {
        if (!(pid in this.#PIDIndexes)) {
            this.#PIDIndexes[pid] = this.#PIDOrder.length
            this.#PIDOrder.push(pid)
            appendChild(this.#headerNode, 'timeline_head_cell', String(pid))
        }
    }

    // adjustPlaceholder adjusts the placeholder for a given timeslot.
    #adjustPlaceholder(timeslot) {
        let slotNode = this.#slotNodes[timeslot]
        if (!slotNode) {
            slotNode = el('timeline_row')
            slotNode.setAttribute('data-timeslot', timeslot)
            slotNode.setAttribute('data-i', timeslot - this.#minTimeslot)
            this.#slotNodes[timeslot] = slotNode
            this.#rootNode.append(slotNode)
        }

        const events = this.data[timeslot] || {}

        // calculate the height of the timeslot block
        const biggestCell = Math.max(...this.#PIDOrder.map(pid => (events[pid] || []).length));
        const height = Math.max(UI.rowHeight*biggestCell, UI.cellHeightMin) + UI.borderHeight;
        slotNode.style.height = height+'px'

        // if the timeslot has any events, show it and observe it
        if (biggestCell > 0) {
            slotNode.style.display = ""
            this.observer.observe(slotNode)
        } else {
            slotNode.style.display = "none"
        }

        if (slotNode.childNodes.length > 0) {
            // if the timeslot is already rendered, update it
            this.#renderContent(timeslot)
        }
    }

    #render() {
        if (this.#minTimeslot === 0) {
            return
        }

        const rootY = this.#rootNode.getBoundingClientRect().top;
        const topOffset = window.scrollY - rootY;
        const windowHeight = window.innerHeight;
        const margin = windowHeight;
        
        console.log('render; top=' + topOffset + '; bottom=' + (topOffset + windowHeight) + '; minTimeslot=' + this.#minTimeslot)

        const toRender = []
        for (let i = 0; i < this.#layout.length; i++) {
            const slot = this.#minTimeslot + i
            const y = this.#layout[i]
            if (slot == this.#currentTimeslot) {
                continue
            }
            if (y > topOffset + windowHeight + margin) {
                toRender.push(i)
                this.#renderContent(slot)
                break
            }
            if (y >= topOffset - margin) {
                toRender.push(i)
                this.#renderContent(slot)
            } else {
                // this.#hideContent(slot)
            }
        }
        console.log('toRender = ' + toRender.join(', '))
        window.layout = this.#layout
        window.slotNodes = this.#slotNodes
        window.minTimeslot = this.#minTimeslot
    }

    #renderContent(timeslot) {
        const node = this.#slotNodes[timeslot]
        if (!node) {
            console.error("can't find node for timeslot=" + timeslot)
            return
        }

        for (let i = 0; i < this.#PIDOrder.length; i++) {
            const pid = this.#PIDOrder[i]
            let cellNode = node.childNodes[i]
            if (!cellNode) {
                cellNode = el('timeline_cell')
                node.append(cellNode)
            }

            let rows = this.data[timeslot] && this.data[timeslot][pid] || []
            for (let j = cellNode.childNodes.length; j < rows.length; j++) {
                const e = rows[j]
                let item = renderStraceItem(e)
                cellNode.append(item)
            }
        }
    }

    #hideContent(timeslot) {
        const node = this.#slotNodes[timeslot]
        if (!node) {
            console.error("can't find node for timeslot=" + timeslot)
            return
        }
        while (node.firstChild) {
            node.removeChild(node.lastChild)
        }
    }
}

(function main(){
    const root = document.querySelector('#main .timeline')
    const timeline = new Timeline(root)
    const eventSource = new EventSource("/events")
    window.timeline = timeline

    eventSource.addEventListener('message', (event) => {
        const e = JSON.parse(event.data)
        // console.log('got eventSource message', e)
        timeline.appendEvent(e)
    })
    eventSource.addEventListener('fin', () => {
        eventSource.close()
        timeline.finish()
    })
    eventSource.onerror = (err) => {
        console.error("EventSource failed:", err)
        eventSource.close()
        timeline.finish()
    }
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

function debounce(callback, wait) {
    let timeoutId = null;
    return (...args) => {
        window.clearTimeout(timeoutId)
        timeoutId = window.setTimeout(callback, wait)
    }
}