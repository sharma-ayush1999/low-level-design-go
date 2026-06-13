package tictactoe

func Run(){
	player1 := NewPlayer("Ayush", 'X')
	player2 := NewPlayer("Rahul", 'O')

	game := NewGame(*player1, *player2)
	game.Play()
}