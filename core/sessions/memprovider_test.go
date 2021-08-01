package sessions

/*
func TestMemStore(t *testing.T) {
	st := NewMemStore()

	for i := 0; i < 10; i++ {
		st.New(fmt.Sprintf("%d", i))
	}

	require.Equal(t, 10, st.Len(), "Incorrect length.")

	for i := 0; i < 10; i++ {
		sess, err := st.Get(fmt.Sprintf("%d", i))
		require.NoError(t, err, "Unable to Get() session #%d", i)

		require.Equal(t, fmt.Sprintf("%d", i), sess.ClientId, "Incorrect ID")
	}

	for i := 0; i < 5; i++ {
		st.Del(fmt.Sprintf("%d", i))
	}

	require.Equal(t, 5, st.Len(), "Incorrect length.")
}
*/
