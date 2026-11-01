import java.util.Properties

plugins {
    id("com.android.application")
}

// Release signing comes from keystore.properties (not in git). Without it,
// release builds fall back to the debug key so anyone can build the APK.
val ks = Properties().apply { rootProject.file("keystore.properties").takeIf { it.exists() }?.inputStream()?.use { load(it) } }

android {
    namespace = "app.goodiecat"
    compileSdk { version = release(37) { minorApiLevel = 2 } }

    defaultConfig {
        applicationId = "app.goodiecat"
        minSdk = 26
        targetSdk = 36
        versionCode = 2
        versionName = "1.1"
    }
    signingConfigs {
        create("release") {
            if (ks.isNotEmpty()) {
                storeFile = file(ks.getProperty("storeFile"))
                storePassword = ks.getProperty("storePassword")
                keyAlias = ks.getProperty("keyAlias")
                keyPassword = ks.getProperty("keyPassword")
            }
        }
    }
    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"))
            signingConfig = signingConfigs.getByName(if (ks.isNotEmpty()) "release" else "debug")
        }
    }
}

dependencies {
    implementation("androidx.swiperefreshlayout:swiperefreshlayout:1.1.0")
}
