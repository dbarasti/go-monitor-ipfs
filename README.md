# Go Monitor IPFS

Application to monitor and create reports with plots regarding several metrics of an IPFS node. 

## Getting Started

These instructions will get you a copy of the project up and running on your local machine for development and testing purposes.

### Prerequisites

* GO version >=1.13


### Installing


First of all let's get you a local copy of the project:

```git clone git@github.com:dbarasti/go-monitor-ipfs.git```   
```cd go-monitor-ipfs```

Then install the dependencies with

```
go get ./...
```

Create and fill a ```.env``` file following the sample ```.env.example``` that you'll find in the project directory.   
unix: ```cp .env.example .env```  
win: ```cp .env.example .env```  
If you followed one of the above two commands you now have to insert appropriate values in the  ```.env``` file.  

Build the app:

```go build```

Now let's run it:

```go run go-monitor-ipfs```


## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING) for details on the code of conduct, and the process for submitting pull requests to the project.


## License

This project is licensed under the GNU General Public License - see the [LICENSE.md](LICENSE) file for details

## Acknowledgments
This project was started for an assignment of the course [Peer to Peer Systems and Blockchains](https://elearning.di.unipi.it/course/info.php?id=118&lang=it) at University of Pisa under the guide of [Prof. Laura Ricci](http://pages.di.unipi.it/ricci/)
