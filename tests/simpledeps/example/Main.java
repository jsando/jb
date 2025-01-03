import com.google.gson.Gson;

public class Main {

    public static class Person {
        private String name;
        private int age;
        private String email;

        public Person(String name, int age, String email) {
            this.name = name;
            this.age = age;
            this.email = email;
        }
    }

    public static void main(String[] args) {
        // Create a Person object
        Person person = new Person("John Doe", 30, "johndoe@example.com");

        // Create a Gson instance
        Gson gson = new Gson();

        // Serialize the Person object to JSON
        String json = gson.toJson(person);

        // Print the JSON string
        System.out.println(json);
    }
}