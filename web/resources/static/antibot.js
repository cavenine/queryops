(function () {
  function applyAntibot(root) {
    var scope = root || document;
    var forms = scope.querySelectorAll("form[data-antibot-token]");

    for (var i = 0; i < forms.length; i++) {
      var form = forms[i];
      var token = form.getAttribute("data-antibot-token");
      if (!token) continue;

      var input = form.querySelector('input[name="js_token"]');
      if (input) input.value = token;

      if (form.getAttribute("data-antibot-listener") !== "true") {
        form.setAttribute("data-antibot-listener", "true");
        form.addEventListener("submit", function (e) {
          var f = e.target;
          if (!f || !f.getAttribute) return;
          var t = f.getAttribute("data-antibot-token");
          if (!t) return;
          var i2 = f.querySelector('input[name="js_token"]');
          if (i2) i2.value = t;
        });
      }
    }
  }

  document.addEventListener("DOMContentLoaded", function () {
    applyAntibot(document);
  });

  // DataStar triggers DOM patches; re-apply after patch.
  document.addEventListener("datastar-fetch", function (e) {
    try {
      if (!e || !e.detail) return;
      if (e.detail.type !== "datastar-patch-elements") return;
      setTimeout(function () {
        applyAntibot(document);
      }, 0);
    } catch (_) {}
  });
})();
