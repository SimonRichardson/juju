(bootstrapping)=
# Boostrapping a model

When you first start looking at the bootstrap process it all seems to be a convoluted mess. However there is method to our madness.

Bootstrapping starts with the CLI command `bootstrap`.  That is found in
   cmd/juju/bootstrap.go

The first step of bootstrap is to create an Environ instance which is named.
This Environ instance has the model configuration (the *config.Config instance).
Initially this will check in the default config store, which is $JUJU_HOME/models.
This calls through to bootstrap.PrepareForName in environs/open.go.  This makes sure that the
model configuration contains an admin secret, a CA cert, and a UUID.

It is at this time that the initial .jenv file is written out to $JUJU_DATA/models.

Further checks are then done as part of the bootstrap command:
 * validating the constaints
 * checking to make sure the model is already bootstrapped

The code then moves on to the Bootstrap function defined in environs/bootstrap/bootstrap.go.

bootstrap.Bootstrap starts with sanity checks:
 * setting a package global in the network package for prefer IPv6 (not sanity)
 * there is an admin-secret
 * that there is at least one authorised SSH key
 * that there is a CA Cert and CA Key
 * that the model storage is writable (by writing the bootstrap-init file)
 * finds available agent binaries
   - locate agent binaries available externally (matching constraints)
   - determine which agent binaries can be built and uploaded to make up shortfall in above
   - if the best agent binaries are local, we attempt to upload them

This code then calls into the Bootstrap function on the environ instance (backed by a provider), which returns arch, series, and a finalizer function.

Now things diverge here a little:
 * azure does some initial config around affinity groups and networks, then calls common.Bootstrap.
 * ec2, maas, and openstack all fall through to common.Bootstrap
 * dummy, local and manual all do their own thing

Firstly, common.Bootstrap:
 * creates machine config for the bootstrap machine
 * starts an instance for the bootstrap machine
 * writes the instance id (as yaml) into the the "provider-state" file in environ storage
   - this step will go away soon, or at least become provider specific

The  finalizer function is run by bootstrap.Bootstrap after the following is performed:
 * select agent binaries from the previously calculated set based on the architecture and series
   of the instance that the provider started
 * make sure that the agent binaries are available
 * create the machine config struct for the bootstrap machine
 * set the agent binaries in that structure to the agent binaries bootstrap knows about.
 
The common finalizer function does the following: 
 * updates the machine config with the instance id of the new machine
 * calls environs.FinishMachineConfig
   * populates the machine config with information from the config object
   * checks for CA Cert
   * checks for admin-secret
   * creates a password hash using the password.CompatSalt
   * uses this password hash for both the APIInfo and MongoInfo passwords.
   * creates the controller cert and key
   * strips the admin-secret and server ca-private-key from the config
     * this step is probably not needed any more
 * calls common.FinishBootstrap
   * calls ssh with a custom script that first checks the nonce on the cloud instance
   * calls ConfigureMachine
     * creates cloud init script from the machine config, this includes the call
       to jujud bootstrap-state.
     * the bootstrap config is passed to jujud as base64 encoded yaml
     * runs said script over ssh

jujud bootstrap-state

 * creates a *config.Config object from the base64 encoded yaml from the command line
 * sets the package global in the network package for prefer IPv6
 * generates and writes out the system SSH identity file
 * generates a (long) shared secret for mongo
 * mongo is then started
 * the database is then initialized (state.Initialize)
 * copies the agent binaries into model storage
   - also clones the agent binaries for each series of the same OS
     (for the time being at least, while each series' agent binaries are equivalent)
