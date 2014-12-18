App.LibrariesIndexRoute = Ember.Route.extend({
  beforeModel: function() {
    this.transitionTo('newLibrary');
  }
});

App.Library = DS.Model.extend({
  name: DS.attr(),
  script: DS.attr()
});

App.LibrariesRoute = Ember.Route.extend({
  model: function() {
    return this.store.find('library');
  }
});

App.LibrariesController = Ember.ArrayController.extend({
  sortProperties: ['name'],
  sortAscending: true
})

App.LibraryRoute = Ember.Route.extend({
  model: function(params) {
    return this.store.find('library', params.library_id);
  }
})

App.LibraryController = Ember.ObjectController.extend({
  needs: ["admin"],

  actions: {
    save: function() {
      var self = this;
      this.model.save().then(function(value) {
        self.set('controllers.admin.successMessage', "Saved!");
        self.set('controllers.admin.errorMessage', null);
      }, function(reason) {
        self.set('controllers.admin.successMessage', null);
        self.set('controllers.admin.errorMessage', reason.responseText);
      });
    },
    delete: function() {
      if (confirm("Delete the endpoint '" + this.model.get('name') + "'?")) {
        this.model.destroyRecord();
        this.set('controllers.admin.successMessage', "Deleted!");
        this.set('controllers.admin.errorMessage', null);
        this.transitionToRoute('libraries');
      }
    }
  }
});

App.NewLibraryRoute = Ember.Route.extend({
  templateName: 'library',
  model: function(params) {
    return this.store.createRecord('library');
  }
})

App.NewLibraryController = Ember.ObjectController.extend({
  // This is almost entirely duplicated from LibraryController,
  // but specifying controllerName in my route wouldn't resolve the
  // 'save' action.

  needs: ["admin"],

  actions: {
    save: function() {
      var self = this;
      this.model.save().then(function(value) {
        self.set('controllers.admin.successMessage', "Created!");
        self.set('controllers.admin.errorMessage', null);
        self.transitionToRoute("library", value.id)
      }, function(reason) {
        self.set('controllers.admin.successMessage', null);
        self.set('controllers.admin.errorMessage', reason.responseText);
      });
    }
  }
});