plugins {
    kotlin("jvm") version "2.0.0"
}

group = "com.boggycreek"
version = "0.1.0-alpha"

repositories {
    mavenCentral()
}

dependencies {
    compileOnly("com.jetbrains.toolbox:remote-dev-api:1.13.87111")
}

tasks.jar {
    manifest {
        attributes(
            "Manifest-Version" to "1.0",
            "Created-By" to "Boggy Creek Software LLC"
        )
    }
}
