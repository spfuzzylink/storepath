(function () {
  "use strict";

  const byId = (id) => document.getElementById(id);
  const clone = (value) => JSON.parse(JSON.stringify(value));
  const escape = (value) => String(value).replace(/[&<>"']/g, (character) => ({"&":"&amp;", "<":"&lt;", ">":"&gt;", '"':"&quot;", "'":"&#39;"})[character]);
  const pretty = (value) => JSON.stringify(value, null, 2);
  const version = String(window.STOREPATH_VERSION || "0.2.0").replace(/^v/, "");
  const productVersion = "0.1.0";
  const releaseURL = "https://github.com/spfuzzylink/storepath/releases/tag/v0.2.0";
  const downloadURL = "https://github.com/spfuzzylink/blockorbucket/releases/download/v0.1.0/blockorbucket-demo.html";
  const presetInfo = [
    {key:"telemetry-retention", title:"Telemetry retention", note:"Active writes + history", description:"Keep active writes on block while retaining sealed time-series data in an object tier. Cache size and origin traffic are separate assumptions."},
    {key:"transactional-db", title:"Transactional DB", note:"Mutable data + WAL", description:"A database with random in-place writes, a filesystem contract, and fsync-style persistence. Compatibility comes before a lower capacity price."},
    {key:"backup-repository", title:"Backup repository", note:"Write once, retain", description:"An immutable backup repository with infrequent retrieval. Compare retained capacity, API requests, retrieval, and delivery costs."},
    {key:"analytics-range-reads", title:"Analytics files", note:"Columnar + range reads", description:"Analytics over immutable columnar files. Byte-range reads fit object APIs; every billed range request and transferred byte still belongs in the budget."},
    {key:"hot-object-service", title:"Hot object service", note:"Read-heavy + cache", description:"A read-heavy service with an explicit block cache and post-cache origin request budget. No cache hit ratio is inferred from cache capacity."},
    {key:"strict-latency", title:"Strict latency", note:"Evidence required", description:"A 5 ms p99 target without measurements. BlockOrBucket keeps the decision open until a compatible candidate has passing evidence."}
  ];
  const names = {block:"Block", object:"Object", hybrid:"Hybrid"};
  const titleNames = {block:"Block storage", object:"Object storage", hybrid:"Hybrid storage"};
  const lineNames = {block_capacity:"Block capacity", block_extra_iops:"Additional block IOPS", block_extra_throughput:"Additional throughput", block_extra_monthly:"Other block costs", object_capacity:"Object capacity", object_get_head:"GET / HEAD requests", object_put_copy_post:"PUT / COPY / POST requests", object_list:"LIST requests", object_retrieval:"Object retrieval", object_egress:"Object egress", object_extra_monthly:"Other object costs"};
  const blockFields = [
    ["copies", "Volume copies", "count", "1"], ["headroom_gib", "Headroom per copy", "GiB"],
    ["provisioned_iops", "IOPS per copy", "IOPS"], ["provisioned_mibps", "Throughput per copy", "MiB/s"],
    ["extra_monthly_cost", "Other block costs", "currency / month"]
  ];
  const accessFields = [
    ["get_requests", "GET / HEAD", "requests / month"], ["put_requests", "PUT / COPY / POST", "requests / month"],
    ["list_requests", "LIST", "requests / month"], ["retrieved_gib", "Retrieved data", "GiB / month"],
    ["egress_gib", "Billable egress", "GiB / month"], ["extra_monthly_cost", "Other object costs", "currency / month"]
  ];
  const rateFields = [
    ["block_gib_month", "Block capacity", "/ GiB-month"], ["object_gib_month", "Object capacity", "/ GiB-month"],
    ["block_included_iops", "Included block IOPS", "IOPS / copy"], ["block_iops_month", "Extra IOPS", "/ IOPS-month"],
    ["block_included_mibps", "Included throughput", "MiB/s / copy"], ["block_mibps_month", "Extra throughput", "/ MiB/s-month"],
    ["get_per_1000", "GET / HEAD", "/ 1,000 requests"], ["put_per_1000", "PUT / COPY / POST", "/ 1,000 requests"],
    ["list_per_1000", "LIST", "/ 1,000 requests"], ["retrieval_gib", "Retrieval", "/ GiB"], ["egress_gib", "Egress", "/ GiB"]
  ];
  let selectedPreset = "telemetry-retention";
  let currentInput = null;
  let lastDecision = null;
  let ready = false;
  let jsonDirty = false;

  function getPath(object, path) {
    return path.split(".").reduce((value, key) => value == null ? undefined : value[key], object);
  }

  function setPath(object, path, value) {
    const keys = path.split(".");
    let target = object;
    keys.slice(0, -1).forEach((key) => {
      if (!target[key]) target[key] = {};
      target = target[key];
    });
    if (value === undefined) delete target[keys[keys.length - 1]];
    else target[keys[keys.length - 1]] = value;
    if (object.workload.measured_p99_ms && !Object.keys(object.workload.measured_p99_ms).length) delete object.workload.measured_p99_ms;
  }

  function number(value, maximumFractionDigits) {
    return new Intl.NumberFormat("en-US", {maximumFractionDigits: maximumFractionDigits === undefined ? 2 : maximumFractionDigits}).format(value);
  }

  function money(value, currency, rate) {
    try {
      return new Intl.NumberFormat("en-US", {style:"currency", currency:currency, minimumFractionDigits:rate ? 0 : 2, maximumFractionDigits:rate ? 6 : 2}).format(value);
    } catch (_) {
      return String(currency) + " " + number(value, rate ? 6 : 2);
    }
  }

  function modeSymbol(mode) {
    const paths = mode === "block" ? '<rect x="4" y="4" width="16" height="16" rx="1"/><path d="M4 10h16M10 4v16"/>' : mode === "object" ? '<ellipse cx="12" cy="6" rx="8" ry="3"/><path d="M4 6v12c0 4 16 4 16 0V6M4 12c0 4 16 4 16 0"/>' : '<rect x="2" y="6" width="9" height="13" rx="1"/><path d="M11 12h3"/><ellipse cx="18" cy="7" rx="5" ry="2"/><path d="M13 7v10c0 3 10 3 10 0V7"/>';
    return '<span class="plan-symbol" aria-hidden="true"><svg viewBox="0 0 26 26" fill="none" stroke-width="1.3">' + paths + '</svg></span>';
  }

  function inputField(prefix, field) {
    const path = prefix + "." + field[0];
    const id = "field-" + path.replace(/\./g, "-");
    return '<label class="field" for="' + id + '"><span>' + escape(field[1]) + '<small>' + escape(field[2]) + '</small></span><input id="' + id + '" data-path="' + path + '" type="number" min="' + (field[0] === "copies" ? "1" : "0") + '" step="' + (field[3] || "any") + '" inputmode="decimal"></label>';
  }

  function fieldGroup(title, prefix, fields) {
    return '<fieldset class="budget-group"><legend>' + escape(title) + '</legend>' + fields.map((field) => inputField(prefix, field)).join("") + '</fieldset>';
  }

  function populateForm() {
    const workload = currentInput.workload;
    let budgets = fieldGroup("Block-only plan", "workload.block", blockFields);
    budgets += fieldGroup("Direct object API · monthly access", "workload.object_access", accessFields);
    if (workload.hybrid) {
      budgets += fieldGroup("Hybrid block / cache plan", "workload.hybrid", [["cache_gib", "Cache capacity", "GiB"]]);
      budgets += fieldGroup("Hybrid block provisioning", "workload.hybrid.block", blockFields);
      budgets += fieldGroup("Hybrid origin · monthly access", "workload.hybrid.object_access", accessFields);
    } else {
      budgets += '<p class="field-help">This input has no hybrid plan. Add an explicit hybrid block/cache plan and origin budget in Full input JSON to model one.</p>';
    }
    byId("budget-fields").innerHTML = budgets;
    byId("rate-fields").innerHTML = rateFields.map((field) => inputField("rates", field)).join("");
    document.querySelectorAll("[data-path]").forEach((control) => {
      const value = getPath(currentInput, control.dataset.path);
      if (control.type === "checkbox") control.checked = Boolean(value);
      else control.value = value === undefined ? "" : String(value);
      control.removeAttribute("aria-invalid");
    });
    byId("rate-kind").textContent = currentInput.rates.illustrative ? "illustrative" : "supplied";
    byId("rate-description").textContent = currentInput.rates.label + " · " + currentInput.rates.currency + " · As of " + currentInput.rates.as_of + ". All rates are supplied inputs; none are fetched.";
    byId("input-json").value = pretty(currentInput);
    byId("json-status").textContent = "";
    byId("quick-controls").disabled = !ready;
    byId("input-json").disabled = !ready;
    byId("apply-json").disabled = !ready;
    jsonDirty = false;
  }

  function clearDecision(message) {
    lastDecision = null;
    byId("result-announcement").textContent = message || "No current decision";
    byId("result-output").removeAttribute("data-status");
    byId("result-output").removeAttribute("data-recommendation");
    byId("export-input").disabled = true;
    byId("export-report").disabled = true;
    byId("result-output").setAttribute("aria-busy", "false");
    byId("result-output").innerHTML = '<div class="empty-state">' + escape(message || "No current decision") + '<small>Update the assumptions to evaluate a new plan.</small></div>';
  }

  function showError(message) {
    clearDecision("Fix the input to continue.");
    byId("result-error").innerHTML = '<strong>These assumptions need a closer look.</strong><span>' + escape(message) + '</span>';
    byId("result-error").hidden = false;
  }

  function hideError() { byId("result-error").hidden = true; byId("result-error").textContent = ""; }

  function latencyText(candidate) {
    if (!candidate.compatible) return "Latency cannot make an incompatible plan eligible.";
    const latency = candidate.latency;
    if (latency.status === "not_requested") return "No latency target supplied.";
    if (latency.status === "unmeasured") return "Unmeasured · target " + number(latency.target_p99_ms) + " ms p99";
    return number(latency.measured_p99_ms) + " ms p99 · " + (latency.status === "passes" ? "meets" : "exceeds") + " " + number(latency.target_p99_ms) + " ms target";
  }

  function candidateCard(candidate, decision) {
    const selected = candidate.mode === decision.recommendation;
    let status = candidate.compatible ? "Compatible" : "Incompatible";
    let statusClass = candidate.compatible ? "" : "negative";
    if (candidate.compatible && candidate.latency.status === "unmeasured") { status = "Needs measurement"; statusClass = "warning"; }
    if (candidate.compatible && candidate.latency.status === "fails") { status = "Latency target failed"; statusClass = "negative"; }
    if (candidate.compatible && candidate.latency.status === "passes") status = "Evidence meets target";
    const price = candidate.cost ? escape(money(candidate.cost.monthly_total, candidate.cost.currency)) + ' <span>/mo</span>' : "—";
    const reasons = candidate.reasons.map((reason) => '<p>' + escape(reason) + '</p>').join("");
    return '<article data-mode="' + candidate.mode + '" class="candidate' + (selected ? " selected" : "") + (!candidate.compatible ? " unavailable" : "") + '"><div class="candidate-name">' + names[candidate.mode] + (selected ? '<span class="candidate-check" aria-label="Selected plan">✓</span>' : "") + '</div><div class="candidate-price' + (!candidate.cost ? " empty" : "") + '">' + price + '</div><div class="candidate-status ' + statusClass + '">' + escape(status) + '</div><details><summary>Why this result</summary><div class="candidate-reasons">' + reasons + '</div></details><div class="candidate-latency">' + escape(latencyText(candidate)) + '</div></article>';
  }

  function costTable(candidate, selected) {
    if (!candidate.cost) return "";
    const cost = candidate.cost;
    const rows = cost.items.map((item) => '<tr><td>' + escape(lineNames[item.name] || item.name) + '</td><td>' + escape(number(item.quantity, 6)) + '<br><span class="quantity-unit">' + escape(item.unit) + '</span></td><td>' + escape(money(item.rate, cost.currency, true)) + '</td><td>' + escape(money(item.cost, cost.currency)) + '</td></tr>').join("");
    return '<details class="cost-details"' + (selected ? " open" : "") + '><summary>' + names[candidate.mode] + ' · every modeled cost' + (selected ? " · selected" : "") + '</summary><div class="table-wrap" role="region" tabindex="0" aria-label="' + names[candidate.mode] + ' itemized monthly costs"><table class="cost-table"><thead><tr><th scope="col">Component</th><th scope="col">Quantity</th><th scope="col">Unit rate</th><th scope="col">Monthly</th></tr></thead><tbody>' + rows + '</tbody><tfoot><tr><td colspan="3">Modeled monthly total</td><td>' + escape(money(cost.monthly_total, cost.currency)) + '</td></tr></tfoot></table></div><p class="table-caption">' + escape(cost.currency) + ' · Totals use unrounded values. Displayed amounts may differ slightly when summed.</p></details>';
  }

  function renderDecision(decision) {
    const recommended = decision.candidates.find((candidate) => candidate.mode === decision.recommendation);
    const pending = decision.status === "benchmark_required";
    const unavailable = decision.status === "no_feasible_plan";
    const heading = recommended ? titleNames[recommended.mode] : pending ? "Benchmark before choosing." : "No eligible plan yet.";
    const status = recommended ? (decision.status === "latency_evidence_satisfied" ? "MEETS SUPPLIED LATENCY EVIDENCE" : "LOWEST MODELED COMPATIBLE COST") : pending ? "EVIDENCE NEEDED" : "REVISIT THE DESIGN";
    let metrics = "";
    if (recommended) {
      const difference = decision.modeled_monthly_difference_from_block;
      metrics = '<div class="recommendation-metrics"><div><span class="metric-label">Modeled monthly storage cost</span><span class="metric-value">' + escape(money(recommended.cost.monthly_total, recommended.cost.currency)) + '<small>/mo</small></span><p class="metric-caption">Only the listed cost components.</p></div>';
      if (typeof difference === "number") {
        metrics += '<div><span class="metric-label">Modeled monthly difference vs. block</span><span class="metric-value">' + escape(money(Math.abs(difference), recommended.cost.currency)) + '<small>' + (difference > 0 ? "lower" : difference < 0 ? "higher" : "difference") + '</small></span><p class="metric-caption">Under these assumptions. Not realized savings.</p></div>';
      }
      metrics += '</div>';
    }
    const assumptionList = decision.assumptions.map((assumption) => '<li>' + escape(assumption) + '</li>').join("");
    const exclusionList = decision.exclusions.map((exclusion) => '<li>' + escape(exclusion) + '</li>').join("");
    const tables = decision.candidates.slice().sort((a, b) => Number(b.mode === decision.recommendation) - Number(a.mode === decision.recommendation)).map((candidate) => costTable(candidate, candidate.mode === decision.recommendation)).join("");
    byId("result-output").innerHTML = '<div class="recommendation' + (pending ? " needs-evidence" : unavailable ? " error-state" : "") + '"><div class="recommendation-top"><div><p class="eyebrow">' + status + '</p><h4>' + heading + '</h4></div>' + (recommended ? modeSymbol(recommended.mode) : '') + '</div><p class="recommendation-reason">' + escape(decision.reason) + '</p>' + metrics + '</div><div class="result-subheading">Three plans. One set of assumptions.<small>' + escape(currentInput.rates.currency) + ' / MONTH</small></div><div class="candidate-grid">' + decision.candidates.map((candidate) => candidateCard(candidate, decision)).join("") + '</div><p class="data-note">Compatibility and latency evidence determine eligibility. Costs do not establish equivalent availability or durability.</p>' + tables + '<details class="assumptions"><summary>Read the assumptions &amp; exclusions</summary><h5>Assumptions</h5><ul>' + assumptionList + '</ul><h5>Excluded unless explicitly supplied</h5><ul>' + exclusionList + '</ul></details><p class="assumption-preview">This is a storage planning prototype. It does not provision storage, run benchmarks, or estimate complete total cost of ownership.</p><p class="rate-meta">' + escape(decision.rate_label) + ' · ' + escape(decision.rate_as_of) + ' · ' + (decision.illustrative ? "Illustrative rates" : "Caller-supplied rates") + '</p>';
    byId("result-output").setAttribute("aria-busy", "false");
    byId("result-output").dataset.status = decision.status;
    byId("result-output").dataset.recommendation = decision.recommendation || "";
    byId("result-announcement").textContent = heading + " " + (recommended ? "Modeled monthly cost: " + money(recommended.cost.monthly_total, recommended.cost.currency) + ". " : "") + decision.reason;
    byId("export-input").disabled = false;
    byId("export-report").disabled = false;
    lastDecision = clone(decision);
    hideError();
  }

  function evaluate(raw, replacingInput) {
    if (!ready) return false;
    clearDecision("Evaluating your assumptions…");
    try {
      // Preserve the original JSON text through the strict Go decoder. Parsing
      // first would erase duplicate keys and bypass that validation contract.
      const result = JSON.parse(window.storepathEvaluate(raw));
      if (result.error) throw new Error(result.error);
      if (!result.decision) throw new Error("The storage engine returned no decision.");
      if (replacingInput) {
        if (!result.input) throw new Error("The storage engine returned no normalized input.");
        currentInput = result.input;
        populateForm();
      }
      renderDecision(result.decision);
      return true;
    } catch (error) {
      showError(error.message || String(error));
      return false;
    }
  }

  function selectPreset(key) {
    const presets = window.STOREPATH_PRESETS || {};
    const preset = presets[key] || presets[key + ".json"];
    if (!preset) { showError("The bundled scenario could not be found."); return; }
    selectedPreset = key;
    currentInput = clone(preset);
    document.querySelectorAll("[data-preset]").forEach((button) => button.setAttribute("aria-pressed", String(button.dataset.preset === key)));
    byId("scenario-description").textContent = presetInfo.find((item) => item.key === key).description;
    populateForm();
    if (ready) evaluate(pretty(currentInput), false);
  }

  function onControlChange(event) {
    const control = event.target;
    if (!control.dataset.path || !currentInput || jsonDirty) return;
    let value;
    if (control.type === "checkbox") value = control.checked;
    else if (control.type === "number") {
      if (control.validity.badInput) {
        control.setAttribute("aria-invalid", "true");
        showError("Enter a valid number for " + (control.closest("label").querySelector("span").textContent) + ".");
        return;
      } else if (control.value === "" && control.dataset.optional) value = undefined;
      else if (control.value === "" || !Number.isFinite(Number(control.value)) || !control.checkValidity()) {
        control.setAttribute("aria-invalid", "true");
        showError("Enter a valid nonnegative number for " + (control.closest("label").querySelector("span").textContent) + ".");
        return;
      } else value = Number(control.value);
    } else value = control.value;
    control.removeAttribute("aria-invalid");
    setPath(currentInput, control.dataset.path, value);
    byId("input-json").value = pretty(currentInput);
    const invalid = document.querySelector('[data-path][aria-invalid="true"]');
    if (invalid) { showError("Complete the highlighted input before evaluating."); return; }
    if (evaluate(pretty(currentInput), false)) {
      byId("json-status").textContent = "";
      if (control.dataset.path.startsWith("rates.")) byId("rate-kind").textContent = currentInput.rates.illustrative ? "illustrative" : "supplied";
    }
  }

  function downloadJSON(suffix, content) {
    if (!lastDecision || jsonDirty) return;
    const filename = String(currentInput.workload.name).toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "").slice(0, 70) || "workload";
    const blob = new Blob([pretty(content) + "\n"], {type:"application/json"});
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = "blockorbucket-" + filename + "-" + suffix + ".json";
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    window.setTimeout(() => URL.revokeObjectURL(url), 1000);
  }

  function engineReady() {
    if (ready) return;
    if (typeof window.storepathEvaluate !== "function") return;
    ready = true;
    byId("engine-status").classList.remove("failed");
    byId("engine-status").innerHTML = '<span class="status-dot"></span><span>Go engine · running locally</span>';
    selectPreset(selectedPreset);
  }

  function engineFailed(message) {
    ready = false;
    byId("engine-status").classList.add("failed");
    byId("engine-status").innerHTML = '<span class="status-dot"></span><span>Engine could not start</span>';
    byId("quick-controls").disabled = true;
    byId("input-json").disabled = true;
    byId("apply-json").disabled = true;
    showError(String(message || "WebAssembly could not start in this browser.") + " Try a current desktop browser, or use the native CLI from the GitHub release.");
  }

  document.querySelectorAll("[data-version]").forEach((node) => { node.textContent = "v" + productVersion + " / preview"; });
  document.querySelectorAll("[data-demo-download]").forEach((anchor) => { anchor.href = downloadURL; });
  document.querySelectorAll("[data-release-link]").forEach((anchor) => { anchor.href = releaseURL; });
  byId("install-command").textContent = "go get github.com/spfuzzylink/blockorbucket@v" + productVersion;
  byId("presets").innerHTML = presetInfo.map((preset, index) => '<button class="preset-button" type="button" data-preset="' + preset.key + '" aria-pressed="false"><span class="preset-number">0' + (index + 1) + '</span><span class="preset-title">' + preset.title + '</span><span class="preset-note">' + preset.note + '</span></button>').join("");
  byId("presets").addEventListener("click", (event) => {
    const button = event.target.closest("[data-preset]");
    if (button) selectPreset(button.dataset.preset);
  });
  byId("reset-input").addEventListener("click", () => selectPreset(selectedPreset));
  byId("workload-form").addEventListener("submit", (event) => event.preventDefault());
  byId("workload-form").addEventListener("input", onControlChange);
  byId("input-json").addEventListener("input", () => {
    jsonDirty = true;
    byId("quick-controls").disabled = true;
    hideError();
    clearDecision("JSON edits are waiting to be applied.");
    byId("json-status").textContent = "Unsaved edits. Apply JSON to update the controls and decision, or Reset to restore this scenario.";
  });
  byId("apply-json").addEventListener("click", () => {
    if (evaluate(byId("input-json").value, true)) byId("json-status").textContent = "Applied. Controls and decision now match this JSON.";
  });
  byId("export-input").addEventListener("click", () => downloadJSON("input", currentInput));
  byId("export-report").addEventListener("click", () => downloadJSON("report", {
    report_schema_version:1,
    product:"BlockOrBucket",
    version:productVersion,
    core_engine:"Storepath",
    core_engine_version:version,
    generated_at:new Date().toISOString(),
    report_note:"Modeled storage costs under supplied assumptions, not realized savings or complete TCO. No provider API was queried and no benchmark was performed.",
    input:currentInput,
    decision:lastDecision
  }));
  window.addEventListener("storepath-ready", engineReady);
  window.addEventListener("storepath-error", (event) => engineFailed(event.detail && (event.detail.message || event.detail)));
  selectPreset(selectedPreset);
  if (window.storepathEngineError) engineFailed(window.storepathEngineError);
  else engineReady();
})();
