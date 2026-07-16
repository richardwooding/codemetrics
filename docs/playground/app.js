/* codemetrics playground — boot the WASM analyzer, re-score on every edit.
   All analysis happens in-page; no source code ever leaves the browser. */
(function () {
  "use strict";

  var LANGS = [
    { id: "go", label: "Go" },
    { id: "python", label: "Python" },
    { id: "javascript", label: "JavaScript" },
    { id: "typescript", label: "TypeScript" },
    { id: "rust", label: "Rust" },
    { id: "java", label: "Java" },
    { id: "c", label: "C" },
    { id: "cpp", label: "C++" },
    { id: "csharp", label: "C#" },
    { id: "ruby", label: "Ruby" },
    { id: "php", label: "PHP" }
  ];

  var SAMPLES = {
    go: 'package demo\n\nfunc classify(n int, tags []string) string {\n\tif n < 0 {\n\t\treturn "negative"\n\t}\n\tout := ""\n\tfor _, t := range tags {\n\t\tswitch {\n\t\tcase t == "":\n\t\t\tcontinue\n\t\tcase n > 10 && t != "big":\n\t\t\tout += t\n\t\tdefault:\n\t\t\tout += "?"\n\t\t}\n\t}\n\tif out == "" {\n\t\treturn "empty"\n\t}\n\treturn out\n}\n',
    python: 'def route(request, handlers, fallback=None):\n    if not request:\n        raise ValueError("empty request")\n    for pattern, handler in handlers.items():\n        if pattern == request.path:\n            return handler(request)\n        elif pattern.endswith("*") and request.path.startswith(pattern[:-1]):\n            try:\n                return handler(request)\n            except TimeoutError:\n                continue\n    if fallback:\n        return fallback(request)\n    return None\n',
    javascript: 'function debounceMerge(events, windowMs) {\n  const merged = [];\n  for (const e of events) {\n    const last = merged[merged.length - 1];\n    if (last && e.ts - last.ts < windowMs && e.type === last.type) {\n      last.count += 1;\n    } else if (e.type === "error" || e.retries > 3) {\n      merged.push({ ...e, count: 1, urgent: true });\n    } else {\n      merged.push({ ...e, count: 1 });\n    }\n  }\n  return merged.filter(m => m.count > 0);\n}\n',
    typescript: 'type Task = { id: string; deps: string[]; done: boolean };\n\nfunction ready(tasks: Task[], failed: Set<string>): Task[] {\n  const done = new Set(tasks.filter(t => t.done).map(t => t.id));\n  return tasks.filter(t => {\n    if (t.done || failed.has(t.id)) {\n      return false;\n    }\n    for (const d of t.deps) {\n      if (!done.has(d) || failed.has(d)) {\n        return false;\n      }\n    }\n    return true;\n  });\n}\n',
    rust: 'fn summarize(values: &[i64], cap: usize) -> String {\n    let mut out = String::new();\n    for (i, v) in values.iter().enumerate() {\n        if i >= cap {\n            out.push(\'…\');\n            break;\n        }\n        match v {\n            0 => out.push(\'0\'),\n            n if *n < 0 => out.push(\'-\'),\n            _ => out.push(\'+\'),\n        }\n    }\n    if out.is_empty() { "empty".into() } else { out }\n}\n',
    java: 'class Router {\n    String dispatch(String path, Map<String, String> routes) {\n        if (path == null || path.isEmpty()) {\n            throw new IllegalArgumentException("path");\n        }\n        for (Map.Entry<String, String> e : routes.entrySet()) {\n            if (e.getKey().equals(path)) {\n                return e.getValue();\n            } else if (e.getKey().endsWith("*")\n                    && path.startsWith(e.getKey().substring(0, e.getKey().length() - 1))) {\n                return e.getValue();\n            }\n        }\n        return routes.getOrDefault("default", "404");\n    }\n}\n',
    c: 'int parse_flags(const char **argv, int argc, int *verbose, int *dry_run) {\n    int seen = 0;\n    for (int i = 1; i < argc; i++) {\n        if (argv[i][0] != \'-\') {\n            continue;\n        }\n        switch (argv[i][1]) {\n        case \'v\': *verbose = 1; seen++; break;\n        case \'n\': *dry_run = 1; seen++; break;\n        default:\n            if (argv[i][1] != \'\\0\') {\n                return -1;\n            }\n        }\n    }\n    return seen;\n}\n',
    cpp: 'std::string join(const std::vector<std::string>& parts,\n                 const std::string& sep, bool skipEmpty) {\n    std::string out;\n    for (size_t i = 0; i < parts.size(); ++i) {\n        if (skipEmpty && parts[i].empty()) {\n            continue;\n        }\n        if (!out.empty()) {\n            out += sep;\n        }\n        out += parts[i];\n    }\n    return out.empty() ? "<none>" : out;\n}\n',
    csharp: 'class Inventory {\n    public int Restock(Dictionary<string, int> stock, IEnumerable<string> orders) {\n        var restocked = 0;\n        foreach (var sku in orders) {\n            if (!stock.TryGetValue(sku, out var count)) {\n                continue;\n            }\n            if (count < 5) {\n                stock[sku] = count + 10;\n                restocked++;\n            } else if (count > 100 && sku.StartsWith("promo-")) {\n                stock[sku] = 50;\n            }\n        }\n        return restocked;\n    }\n}\n',
    ruby: 'def tally(entries, min: 1)\n  totals = Hash.new(0)\n  entries.each do |e|\n    next if e.nil? || e.empty?\n    if e.start_with?("#")\n      totals[:comments] += 1\n    elsif e.include?("=")\n      key, value = e.split("=", 2)\n      totals[key.strip.to_sym] += value.to_i\n    else\n      totals[:other] += 1\n    end\n  end\n  totals.select { |_, v| v >= min }\nend\n',
    php: '<?php\nfunction normalize(array $rows, bool $strict = false): array {\n    $out = [];\n    foreach ($rows as $row) {\n        if (!is_array($row) || empty($row)) {\n            if ($strict) {\n                throw new InvalidArgumentException("bad row");\n            }\n            continue;\n        }\n        foreach ($row as $key => $value) {\n            $out[strtolower($key)] = is_string($value) ? trim($value) : $value;\n        }\n    }\n    return $out;\n}\n'
  };

  var $ = function (id) { return document.getElementById(id); };
  var editor = $("src"), status = $("status"), rows = $("rows"), langsBox = $("langs");
  var current = "go";
  var edited = {}; // per-language buffer once the user types

  // --- language pills -------------------------------------------------------
  LANGS.forEach(function (l) {
    var b = document.createElement("button");
    b.className = "gl-lang";
    b.type = "button";
    b.textContent = l.label;
    b.setAttribute("data-gl-tab", l.id); // gloam pill styling for interactive pills
    b.setAttribute("aria-pressed", String(l.id === current));
    b.addEventListener("click", function () { switchLang(l.id); });
    langsBox.appendChild(b);
  });

  function switchLang(id) {
    edited[current] = editor.value;
    current = id;
    Array.prototype.forEach.call(langsBox.children, function (b) {
      b.setAttribute("aria-pressed", String(b.getAttribute("data-gl-tab") === id));
      b.setAttribute("aria-selected", String(b.getAttribute("data-gl-tab") === id));
    });
    editor.value = edited[id] != null ? edited[id] : SAMPLES[id];
    analyzeNow();
  }

  // --- boot the Go runtime --------------------------------------------------
  var go = new Go();
  var boot = (WebAssembly.instantiateStreaming
    ? WebAssembly.instantiateStreaming(fetch("codemetrics.wasm"), go.importObject)
    : fetch("codemetrics.wasm").then(function (r) { return r.arrayBuffer(); })
        .then(function (b) { return WebAssembly.instantiate(b, go.importObject); }))
    .then(function (result) {
      go.run(result.instance);
      return new Promise(function (resolve) {
        (function wait() {
          if (typeof window.cmAnalyze === "function") return resolve();
          setTimeout(wait, 10);
        })();
      });
    });

  boot.then(function () {
    editor.value = SAMPLES[current];
    analyzeNow();
  }).catch(function (err) {
    status.textContent = "could not load codemetrics.wasm — " + String(err);
    status.className = "gl-hint err";
  });

  // --- analyze on edit ------------------------------------------------------
  var timer = null;
  editor.addEventListener("input", function () {
    clearTimeout(timer);
    timer = setTimeout(analyzeNow, 250);
  });
  editor.addEventListener("keydown", function (e) {
    if (e.key === "Tab") {
      e.preventDefault();
      var s = editor.selectionStart, t = editor.selectionEnd;
      editor.setRangeText(current === "go" ? "\t" : "    ", s, t, "end");
      editor.dispatchEvent(new Event("input"));
    }
  });

  function analyzeNow() {
    if (typeof window.cmAnalyze !== "function") return;
    var res;
    try {
      res = JSON.parse(window.cmAnalyze(current, editor.value));
    } catch (err) {
      res = { error: "analyzer crashed: " + String(err) };
    }
    render(res);
  }

  function scoreCell(value, warnAt, badAt) {
    var td = document.createElement("td");
    td.className = "num";
    if (value == null) { td.textContent = "—"; return td; }
    var span = document.createElement("span");
    span.className = "score " + (value > badAt ? "bad" : value > warnAt ? "warn" : "ok");
    span.textContent = String(value);
    td.appendChild(span);
    return td;
  }

  function render(res) {
    rows.textContent = "";
    if (res.error) {
      status.textContent = res.error;
      status.className = "gl-hint err";
      return;
    }
    var fns = res.functions || [];
    status.className = "gl-hint";
    status.textContent = fns.length
      ? fns.length + (fns.length === 1 ? " function" : " functions") + " · re-scored as you type"
      : "no functions found yet — paste or type some code";
    fns.forEach(function (f) {
      var tr = document.createElement("tr");
      var fn = document.createElement("td");
      fn.className = "fn";
      fn.textContent = f.name || "(anonymous)";
      var loc = document.createElement("div");
      loc.className = "loc";
      loc.textContent = "lines " + f.startLine + "–" + f.endLine;
      fn.appendChild(loc);
      tr.appendChild(fn);
      var lines = document.createElement("td");
      lines.className = "num";
      lines.textContent = String(f.lines);
      tr.appendChild(lines);
      tr.appendChild(scoreCell(f.cyclomatic, 10, 20));
      tr.appendChild(scoreCell(f.cognitive, 7, 15));
      rows.appendChild(tr);
    });
  }
})();
