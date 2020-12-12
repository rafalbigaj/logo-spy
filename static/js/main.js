(function($, global) {

  function App() {
  }

  App.prototype = {
    clients: {},
    records: {},
    lastRecords: [],
    employees: {},
    employeeNames: {},
    hourlyGross: 90,
    employee: global.employee,

    loadClients: function() {
      var self = this;
      return $.get("/clients").done(function(clients) {
        self.clients = mapById(clients);
      });
    },

    loadEmployees: function() {
      var self = this;
      if (this.employee && this.employee.admin) {
        return $.get("/employees").done(function(employees) {
          self.employees = mapById(employees);
          self.employeeNames = _.reduce(employees,
            function(map, employee) { map[employee.id] = employee.name; return map; }, {});
        });
      } else {
        return $.get("/employees", {"only-names": true}).done(function(employeeNames) {
          self.employeeNames = employeeNames;
        });
      }
    },

    loadRecords: function() {
      var self = this;
      return $.get("/records", "json").done(function(records) {
        self.records = mapById(records);
        self.lastRecords = records;
      });
    },

    loadData: function() {
      var self = this;
      return $.when(this.loadClients(), this.loadEmployees(), this.loadRecords()).done(function() {
        self.resolveRelations();
      });
    },

    resolveRelations: function() {
      var self = this;
      _.each(this.lastRecords, function(record) {
        record.client = self.clients[record.clientId];
        record.employeeName = self.employeeNames[record.employeeId];
      });
    }
  };

  var app = new App();
  global.app = app;

  $('body').on("refresh", function(_, data) {
    $(document.body).toggleClass('admin', app.employee && app.employee.admin);
    if (!app.employee) {
      $("#login-container").show();
      $("#main-container").hide();
    } else {
      $("#login-container").hide();
      $("#main-container").show();

      app.loadData().done(function() {
        $('#records').trigger('refresh');
        $('#clients').trigger('refresh');
        $('#employees').trigger('refresh');
      });
    }
  });

  $(".form-signin button").click(function() {
    var $this = $(this);
    $this.prop("disabled", true);
    $('.js-signin .alert').fadeOut();
    $.post("/login", $(".js-signin").serialize(), null, 'json')
      .done(function(employee) {
        app.employee = employee;
        $('body').trigger('refresh');
      })
      .fail(function() {
        $('.js-signin .alert').fadeIn();
      })
      .always(function() {
        $this.prop("disabled", false);
      });
    return false;
  });

  $(".js-signout").click(function() {
    $.get("/logout")
      .done(function() {
        app.employee = null;
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
    app.loadClients().done(function(clients) {
      var compiled = _.template($panel.find("script").text());
      var items = _.map(app.clients, function(client) { return compiled(client) });
      $panel.find(".items").html(items.join("\n"));
    });
    return false; // stop propagation
  });

  $('.js-add-client').on('show.bs.modal', function (event) {
    var $link = $(event.relatedTarget);
    if ($link.length > 0) { // triggered by button not datepicker
      var $form = $(this).find('form');
      var client_id = $link.data('id'); // Extract client to be updated
      var client = {};
      if (client_id) {
        client = app.clients[client_id];
        if (!client) {
          console.error("Cannot find client with id: " + client_id);
        }
      }
      populateForm($form, client);
     }
  });

  $(".js-add-client button.js-save").click(function() {
    var $form = $('.js-add-client form');
    var json = $form.serializeJSON();
    var client_id = $form.data('object-id');
    var existing = client_id && client_id != '';
    var type = existing ? 'POST' : 'PUT';
    var url = '/clients';
    if (existing) {
      url += '/' + client_id;
    }
    $.ajax({
      url: url,
      type: type,
      data: JSON.stringify(json)
    }).done(function() {
        $("#clients").trigger('refresh');
      }).always(function() {
        $('.js-add-client').modal('hide');
      })
  });

  $(".js-add-client button.js-remove").click(function() {
    var $form = $('.js-add-client form');
    var client_id = $form.data('object-id');
    $.ajax({
      url: '/clients/'+client_id,
      type: 'DELETE'
    }).done(function() {
      $("#clients").trigger('refresh');
    }).always(function() {
      $('.js-add-client').modal('hide');
    });
  });

  /** records */

  $("#records").on('refresh', function() {
    var $panel = $(this);
    app.loadData().done(function() {
        var compiled = _.template($panel.find("script").text());
        var items = _.map(app.lastRecords, function(record) {
          if (record.client===undefined) {
            record.client = {id: record.clientId};
          }
          return compiled(record);
        });
        $panel.find(".items").html(items.join("\n"));
      });
    return false; // stop propagation
  });

  $('.js-record-modal').on('show.bs.modal', function (event) {
    var $link = $(event.relatedTarget);
    if ($link.length > 0) { // triggered by button not datepicker
      var $form = $(this).find('form');
      var record_id = $link.data('id'); // Extract client to be updated
      var now = new Date();
      var record = {
        date: formatDateTime(now),
        employeeIncome: app.employee.hourlyNet,
        price: app.hourlyGross
      };
      var $clients_select = $form.find("select#recordClient");
      fillClientsSelect($clients_select);
      if (record_id) {
        record = app.records[record_id];
        if (!record) {
          console.error("Cannot find record with id: " + record_id);
        }
      }
      populateForm($form, record);
     }
  });

  $('.js-record-modal form select#recordClient').change(function() {
    var client_id = $(this).val();
    var client = app.clients[client_id];
    if (client) {
      var price = client.specialPrice || app.hourlyGross;
      $('.js-record-modal form input#recordPrice').val(price);
    } else {
      console.error("Cannot find client with id: " + client_id)
    }
  });

  $(".js-record-modal button.js-remove").click(function() {
    var $form = $('.js-record-modal form');
    var record_id = $form.data('object-id');
    $.ajax({
      url: '/records/'+record_id,
      type: 'DELETE'
    }).done(function() {
      $("#records").trigger('refresh');
    }).always(function() {
      $('.js-record-modal').modal('hide');
    });
  });

  function fillClientsSelect($select) {
    $select.empty();
    _.each(app.clients, function(client) {
      $select.append("<option value='"+client.id+"'>"+client.name+"</option>");
    });
  }

  $(".js-record-modal button.js-save").click(function() {
    var $form = $('.js-record-modal form');
    var json = $form.serializeJSON();
    json['employeeId'] = app.employee.id;
    var record_id = $form.data('object-id');
    var existing = (record_id && record_id != '');
    var type = existing ? 'POST' : 'PUT';
    var url = '/records';
    if (existing) {
      url += '/' + record_id;
    }
    $.ajax({
      url: url,
      type: type,
      data: JSON.stringify(json)
    }).done(function() {
      $("#records").trigger('refresh');
    }).always(function() {
      $('.js-record-modal').modal('hide');
    });
  });

  /** employees */

  $("#employees").on('refresh', function() {
    var $panel = $(this);
    app.loadEmployees().done(function() {
      var compiled = _.template($panel.find("script").text());
      var items = _.map(app.employees, function(employee) { return compiled(employee) });
      $panel.find(".items").html(items.join("\n"));
    });
    return false; // stop propagation
  });

  $('.js-employee-modal').on('show.bs.modal', function (event) {
    var $link = $(event.relatedTarget);
    if ($link.length > 0) { // triggered by button not datepicker
      var $form = $(this).find('form');
      var employee_id = $link.data('id'); // Extract client to be updated
      var employee = {};
      if (employee_id) {
        employee = app.employees[employee_id];
        if (!employee) {
          console.error("Cannot find employee with id: " + employee_id);
        }
      }
      populateForm($form, employee);
     }
  });

  $(".js-employee-modal button.js-save").click(function() {
    var $form = $('.js-employee-modal form');
    var json = $form.serializeJSON();
    var employee_id = $form.data('object-id');
    var existing = employee_id && employee_id != '';
    var type = existing ? 'POST' : 'PUT';
    var url = '/employees';
    if (existing) {
      url += '/' + employee_id;
    }
    $.ajax({
      url: url,
      type: type,
      data: JSON.stringify(json)
    }).done(function() {
        $("#employees").trigger('refresh');
      }).always(function() {
        $('.js-employee-modal').modal('hide');
      })
  });

  $(".js-employee-modal button.js-remove").click(function() {
    var $form = $('.js-employee-modal form');
    var employee_id = $form.data('object-id');
    $.ajax({
      url: '/employees/'+employee_id,
      type: 'DELETE'
    }).done(function() {
      $("#employees").trigger('refresh');
    }).always(function() {
      $('.js-employee-modal').modal('hide');
    });
  });  /** common */

  $(".date-picker").each(function() {
    var $this = $(this);
    var only_date = ($this.attr('data-picker') == 'date-only');
    var start_view = only_date ? 2 : 1;
    var min_view = only_date ? 2 : 1;
    var format = only_date ? "yyyy-mm-dd" : "yyyy-mm-dd - hh:ii";
    $this.datetimepicker({
        format: format,
        autoclose: true,
        todayBtn: true,
        startView: start_view,
        minView: min_view,
        weekStart: 1
    });
  });

  $(function() {
    $('body').trigger('refresh');
  });

  function mapById(collection) {
    return _.reduce(collection, function(map, item) { map[item.id] = item; return map; }, {});
  }

  function formatDateTime(d) {
    return d.getFullYear() + '-' + padDateNum(d.getMonth()+1) + '-' + padDateNum(d.getDate()) +
      ' - ' + padDateNum(d.getHours()) + ':00';
  }

  function padDateNum(num) {
    return num < 10 ? '0'+num : num.toString();
  }

})(jQuery, window);

function printAge(birthday) {
  try {
    if (birthday=="") {
      return "";
    }
    var now = moment();
    var b = moment(birthday);
    return now.diff(b, 'years') + " years";
  } catch(e) {
    console.error(e);
    return '?';
  }
}

function populateForm($form, data, opts) {
  $form.data('object-id', data.id || null);
  if (!opts || opts.reset !== false) {
    resetForm($form);
  }

  function getData(name) {
    var key = name.split(/\[|\]/)
    var value = data[key[0]];
    for (var i=1; value && i<key.length; i++) {
      if (key[i]!="") {
        value = value[key[i]];
      }
    }
    return value;
  }

  $form.find('input, select, textarea').each(function() {
    if (this.name) {
      var name = this.name.split(':')[0]
      var value = getData(name);
      if (this.type == "checkbox") {
        $(this).prop('checked', value);
      } else {
        $(this).val(value);
      }
    }
  });
}

function resetForm($form)
{
  $form.find('input, select, textarea').filter(":not(:checkbox, :radio)").val('');
  $form.find('input:radio, input:checkbox').removeAttr('checked').removeAttr('selected');
}
