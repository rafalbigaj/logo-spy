(function($, global) {
  $('body').on("refresh", function(_, data) {
    if (global.employee===undefined) {
      $("#login-container").show();
      $("#main-container").hide();
    } else {
      $("#login-container").hide();
      $("#main-container").show();
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

  $("nav a.js-collection").click(function() {
    var $this = $(this);
    var $parent = $this.parent('li');
    var $target = $($this.attr('data-target'));
    $("nav li").removeClass('active');
    $parent.addClass('active');
    $("#navbar-collapse-1").collapse('hide');
    $(".panel").hide();
    $target.show();
    $target.trigger('refresh');
  });

  $("#clients").on('refresh', function() {
    var $panel = $(this);
    $.get("/clients", "json")
      .done(function(clients) {
        var compiled = _.template($panel.find("script").text());
        var items = _.map(clients, function(client) { return compiled(client) });
        $panel.find("items").html(items.join("\n"));
      });
  });

  $(".js-add-client button.btn-primary").click(function() {
    var json = $('.js-add-client form').serializeJSON();
    $.ajax({
      url: '/clients',
      type: 'PUT',
      data: json
    }).done(function() {
        $("#clients").trigger('refresh');
      });
  });

  $(function() { $('body').trigger('refresh'); });

})(jQuery, window);

function printAge(birthday) {
  var now = moment();
  var b = moment(birthday);
  return now.diff(b, 'years') + " years";
}