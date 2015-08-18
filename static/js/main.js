(function($, global) {
  $('body').on("refresh", function() {
    if (global.employee===undefined) {
      $("#login-container").removeClass('hidden');
      $("#main-container").addClass('hidden')
    } else {
      $("#login-container").addClass('hidden');
      $("#main-container").removeClass('hidden')
    }
  });

  $(".form-signin button").click(function() {
    var $this = $(this);
    $this.prop("disabled", true);
    $.post("/login", $(".js-signin").serialize(), null, 'json')
      .done(function(data) {
        global.employee = data;
        $('body').trigger('refresh');
      })
      .always(function() {
        $this.prop("disabled", false);
      })
  });

  $(".js-signout").click(function() {
    $.get("/logout")
      .done(function() {
        delete global.employee;
      })
      .always(function() {
        $('body').trigger('refresh');
      })
  });

  $(function() { $('body').trigger('refresh'); });

})(jQuery, window);