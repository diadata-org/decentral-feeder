dockerfile="Dockerfile-diaDecentralOracleService"                                                                                                                                                                                           
imageName="diadecentraloracleservice"                                              
type="oracles"                                                                     
version="v1.0.12"                                                                   
build_and_push() {                                                                 
        docker build --build-arg="GITHUB_TOKEN=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" -f "build/$1" -t "diadata.$2" .
        docker tag "diadata.$2" "us.icr.io/dia-registry/$3/$2:latest"              
        docker push "us.icr.io/dia-registry/$3/$2:latest"                          
                                                                                   
        docker tag "diadata.$2" "us.icr.io/dia-registry/$3/$2:$version"            
        docker push "us.icr.io/dia-registry/$3/$2:$version"                        
}                                                                                  
                                                                                   
build_and_push "$dockerfile" "$imageName" "$type"
