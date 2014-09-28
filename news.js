$(document).ready(function() {
  var currentURL = "http://nytimes.com";

  melange.downloadMessage("news@airdispatch.me", "latest", function(msg) {
    console.log(msg);
    $("#title").html(msg.components["airdispat.ch/news/headline"].string);
    $('#background').css('background-image', 'url(http://data.melange:7776/' + msg.components["airdispat.ch/news/image"].string + ')');
    currentURL = msg.components["airdispat.ch/news/url"].string
  });

  $("#background *").click(function() {
    melange.openLink(currentURL);
  });
});
