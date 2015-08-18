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
    $('.js-signin .alert').fadeOut();
    $.post("/login", $(".js-signin").serialize(), null, 'json')
      .done(function(data) {
        global.employee = data;
        $('body').trigger('refresh');
      })
      .fail(function() {
        $('.js-signin .alert').fadeIn();
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

  $(".js-clients").click(function() {
    $.get("/clients", "json")
      .done(function(clients) {
        var compiled = _.template($(".js-clients script").text());
        var rows = _.map(clients, function(client) { return compiled(client) });
        $(".js-clients table tbody").html(rows.join("\n"));
      });
  });

  $(function() { $('body').trigger('refresh'); });

})(jQuery, window);

function printAge(birthday) {
  var now = moment();
  var b = moment(birthday);
  return now.diff(b, 'years') + " years";
}