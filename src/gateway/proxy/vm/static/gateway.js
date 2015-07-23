/**
 * AP is the root namespace for all Gateway-provided functionality.
 *
 * @namespace
 */
var AP = AP || {};

AP.prepareRequests = function() {
  var requests = [];
  var numCalls = arguments.length;
  for (var i = 0; i < numCalls; i++) {
    var call = arguments[i];
    if (!call.request) {
      requests.push(request);
    } else {
      requests.push(call.request);
    }
  }
  return JSON.stringify(requests);
}

AP.insertResponses = function(calls, responses) {
  var numCalls = calls.length;
  for (var i = 0; i < numCalls; i++) {
    var call = calls[i];
    call.response = responses[i];

    if (numCalls == 1) {
      response = call.response;
    }
  }
}

var Log = "";
function takeOverConsole() {
  function intercept(method) {
    var original = console[method];
    console[method] = function() {
      Log += Array.prototype.slice.apply(arguments).join(' ') + "\n";
      original.apply(console, arguments);
    };
  }
  var methods = ['log', 'warn', 'error'];
  for (var i = 0; i < methods.length; i++) {
    intercept(methods[i]);
  }
}
takeOverConsole();
