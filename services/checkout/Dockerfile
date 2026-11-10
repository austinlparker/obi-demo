# Build context is the repo root so the shared proto/ folder is available.
FROM maven:3.10-eclipse-temurin-25 AS build
WORKDIR /src/services/checkout
COPY services/checkout/pom.xml .
RUN mvn -B -q dependency:go-offline
COPY proto /src/proto
COPY services/checkout/src ./src
RUN mvn -B -q package -DskipTests

FROM eclipse-temurin:25-jre
COPY --from=build /src/services/checkout/target/checkout.jar /app/checkout.jar
EXPOSE 50051
ENTRYPOINT ["java", "--enable-native-access=ALL-UNNAMED", "-jar", "/app/checkout.jar"]
