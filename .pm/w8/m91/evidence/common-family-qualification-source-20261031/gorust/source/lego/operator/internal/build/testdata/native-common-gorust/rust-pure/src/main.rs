fn main() {
    let text = "Bex 雪 λ🙂";
    let square = (257_i64 - 1).pow(2);
    let reversed: String = text.chars().rev().collect();
    println!("rust-pure|{}|{}|{}|{}", text.chars().count(), text.len(), square, reversed);
}
